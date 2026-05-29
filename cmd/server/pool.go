package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

const (
	minWaitMs      = 50
	warmWaitMult   = 2
	coldStartEstMs = 700
	minPoolSize    = 2
	maxPoolSize    = 5
	resizeEvery    = 10 // resize check every N invocations
)

type Container struct {
	ID   string
	Port int
}

type FunctionPool struct {
	idle       chan *Container
	mu         sync.Mutex
	size       int
	invocations int64
	nextPort   int
}

type PoolManager struct {
	mu       sync.Mutex
	pools    map[string]*FunctionPool
	nextPort int
}

type RunRequest struct {
	Code  string                 `json:"code"`
	Event map[string]interface{} `json:"event"`
}

type RunResponse struct {
	Status string `json:"status"`
	Output string `json:"output"`
}

type InvokeResult struct {
	Output   string
	Duration time.Duration
	WarmHit  bool
}

func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools:    make(map[string]*FunctionPool),
		nextPort: 9100,
	}
}

func (pm *PoolManager) allocPort() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p := pm.nextPort
	pm.nextPort++
	return p
}

func spawnContainer(port int) (*Container, error) {
	cmd := exec.Command("docker", "run", "-d",
		"--memory", "128m",
		"--cpus", "0.5",
		"--network", "bridge",
		"-p", fmt.Sprintf("%d:9000", port),
		"faas-runner",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	id := string(bytes.TrimSpace(out))

	url := fmt.Sprintf("http://localhost:%d/run", port)
	for i := 0; i < 20; i++ {
		time.Sleep(300 * time.Millisecond)
		resp, err := http.Post(url, "application/json",
			bytes.NewBufferString(`{"code":"def handler(event):\n    return 1"}`))
		if err == nil {
			resp.Body.Close()
			return &Container{ID: id, Port: port}, nil
		}
	}
	return nil, fmt.Errorf("container %s never became ready", id[:12])
}

func (pm *PoolManager) getOrCreatePool(name string) *FunctionPool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if fp, ok := pm.pools[name]; ok {
		return fp
	}

	// lazy init — create pool with minPoolSize containers
	log.Printf("[pool:%s] initializing with %d containers", name, minPoolSize)
	fp := &FunctionPool{
		idle: make(chan *Container, maxPoolSize),
		size: minPoolSize,
	}

	for i := 0; i < minPoolSize; i++ {
		port := pm.nextPort
		pm.nextPort++
		c, err := spawnContainer(port)
		if err != nil {
			log.Fatalf("[pool:%s] failed to spawn container: %v", name, err)
		}
		fp.idle <- c
		log.Printf("[pool:%s] container %s ready on port %d", name, c.ID[:12], port)
	}

	pm.pools[name] = fp
	return fp
}

// targetSize computes desired pool size based on invocation count
func targetSize(invocations int64) int {
	size := int(invocations/10) + minPoolSize
	if size > maxPoolSize {
		size = maxPoolSize
	}
	if size < minPoolSize {
		size = minPoolSize
	}
	return size
}

func (pm *PoolManager) maybeResize(name string, fp *FunctionPool) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	fp.invocations++

	// only check resize every resizeEvery invocations
	if fp.invocations%resizeEvery != 0 {
		return
	}

	desired := targetSize(fp.invocations)
	if desired <= fp.size {
		return
	}

	// grow pool
	toAdd := desired - fp.size
	log.Printf("[pool:%s] resizing %d → %d (invocations=%d)", name, fp.size, desired, fp.invocations)
	for i := 0; i < toAdd; i++ {
		port := pm.allocPort()
		c, err := spawnContainer(port)
		if err != nil {
			log.Printf("[pool:%s] failed to spawn extra container: %v", name, err)
			return
		}
		fp.idle <- c
		log.Printf("[pool:%s] added container %s on port %d", name, c.ID[:12], port)
	}
	fp.size = desired
}

func runOnContainer(c *Container, code string, event map[string]interface{}, start time.Time) (string, time.Duration, error) {
	body, _ := json.Marshal(RunRequest{Code: code, Event: event})
	url := fmt.Sprintf("http://localhost:%d/run", c.Port)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("runner error: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	raw, _ := io.ReadAll(resp.Body)

	var result RunResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", duration, fmt.Errorf("bad runner response: %w", err)
	}

	return result.Output, duration, nil
}

func warmWaitBudget(avgMs int64) time.Duration {
	if avgMs == 0 {
		return minWaitMs * time.Millisecond
	}
	wait := avgMs * warmWaitMult
	if wait >= coldStartEstMs {
		log.Printf("[pool] slow function (avg %dms >= %dms), cold starting immediately", avgMs, coldStartEstMs)
		return 0
	}
	if wait < minWaitMs {
		wait = minWaitMs
	}
	return time.Duration(wait) * time.Millisecond
}

func (pm *PoolManager) Invoke(name string, code string, event map[string]interface{}, avgMs int64) (InvokeResult, error) {
	fp := pm.getOrCreatePool(name)
	start := time.Now()
	budget := warmWaitBudget(avgMs)

	log.Printf("[pool:%s] warm wait budget: %v (avg %dms)", name, budget, avgMs)

	// try warm container
	if budget > 0 {
		select {
		case c := <-fp.idle:
			defer func() { fp.idle <- c }()
			log.Printf("[pool:%s] warm hit", name)
			go pm.maybeResize(name, fp)
			output, duration, err := runOnContainer(c, code, event, start)
			return InvokeResult{Output: output, Duration: duration, WarmHit: true}, err
		case <-time.After(budget):
			log.Printf("[pool:%s] warm wait timed out after %v, cold starting", name, budget)
		}
	}

	// cold start
	port := pm.allocPort()
	log.Printf("[pool:%s] cold starting on port %d", name, port)
	c, err := spawnContainer(port)
	if err != nil {
		return InvokeResult{}, fmt.Errorf("cold start failed: %w", err)
	}

	defer func() {
		exec.Command("docker", "stop", c.ID).Run()
		exec.Command("docker", "rm", c.ID).Run()
		log.Printf("[pool:%s] cold container %s cleaned up", name, c.ID[:12])
	}()

	go pm.maybeResize(name, fp)
	output, duration, err := runOnContainer(c, code, event, start)
	return InvokeResult{Output: output, Duration: duration, WarmHit: false}, err
}

func (pm *PoolManager) Shutdown() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for name, fp := range pm.pools {
		close(fp.idle)
		for c := range fp.idle {
			exec.Command("docker", "stop", c.ID).Run()
			log.Printf("[pool:%s] stopped container %s", name, c.ID[:12])
		}
	}
}
