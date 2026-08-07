// bench 是框架后端的轻量 HTTP 压测工具（标准库实现，无第三方依赖）：
// 支持并发、自定义 header/body，输出 RPS、平均延迟、p50/p95/p99 与错误率。
// 用途：建立可重复的性能基线（配合 README.md 的两个推荐场景），
// 对比优化前后的量化收益，解锁"需要压测数据再决策"的优化项。
//
// 用法示例：
//
//	# 后台列表页（带管理员 token）
//	go run ./scripts/bench -url http://localhost:9902/admin/user.Admin/index \
//	    -n 2000 -c 50 -header 'batoken: <token>' -header 'think-lang: zh-cn'
//
//	# 前台认证接口（带会员 token）
//	go run ./scripts/bench -url http://localhost:9902/api/user/checkIn \
//	    -n 2000 -c 50 -header 'ba-user-token: <token>'
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var (
		url      = flag.String("url", "", "target URL (required)")
		requests = flag.Int("n", 1000, "total number of requests")
		concurr  = flag.Int("c", 50, "number of concurrent workers")
		method   = flag.String("method", "GET", "HTTP method")
		body     = flag.String("body", "", "request body (raw)")
		headers  = flag.String("header", "", "repeatable: 'Key: Value' (comma-separated)")
		timeout  = flag.Duration("timeout", 30*time.Second, "per-request timeout")
	)
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(2)
	}
	if *requests <= 0 || *concurr <= 0 {
		fmt.Fprintln(os.Stderr, "error: -n and -c must be positive")
		os.Exit(2)
	}

	// 解析 header 列表："batoken: x, think-lang: zh-cn"
	var hdr http.Header = make(http.Header)
	for _, part := range strings.Split(*headers, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "error: malformed header %q\n", part)
			os.Exit(2)
		}
		hdr.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	var bodyReader *bytes.Reader
	if *body != "" {
		bodyReader = bytes.NewReader([]byte(*body))
	}

	var (
		start    = time.Now()
		wg       sync.WaitGroup
		durations = make([]time.Duration, 0, *requests)
		mu       sync.Mutex
		success  atomic.Int64
		failed   atomic.Int64
	)
	perWorker := (*requests + *concurr - 1) / *concurr

	client := &http.Client{Timeout: *timeout}

	worker := func() {
		defer wg.Done()
		for i := 0; i < perWorker; i++ {
			reqBody := io.Reader(nil)
			if bodyReader != nil {
				reqBody = bytes.NewReader([]byte(*body))
			}
			req, err := http.NewRequestWithContext(context.Background(), *method, *url, reqBody)
			if err != nil {
				failed.Add(1)
				continue
			}
			req.Header = hdr.Clone()

			t0 := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(t0)
			mu.Lock()
			durations = append(durations, elapsed)
			mu.Unlock()

			if err != nil {
				failed.Add(1)
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				success.Add(1)
			} else {
				failed.Add(1)
			}
		}
	}

	for w := 0; w < *concurr; w++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()
	elapsedTotal := time.Since(start)

	mu.Lock()
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	total := len(durations)
	mu.Unlock()

	if total == 0 {
		fmt.Fprintln(os.Stderr, "no requests completed")
		os.Exit(1)
	}
	pct := func(p float64) time.Duration {
		idx := int(float64(total-1) * p)
		return durations[idx]
	}

	totalInt := int64(total)
	rps := float64(totalInt) / elapsedTotal.Seconds()
	fmt.Printf("target   : %s %s\n", *method, *url)
	fmt.Printf("requests : %d (concurrency %d)\n", total, *concurr)
	fmt.Printf("success  : %d, failed: %d\n", success.Load(), failed.Load())
	fmt.Printf("elapsed  : %s\n", elapsedTotal.Round(time.Millisecond))
	fmt.Printf("rps      : %.1f req/s\n", rps)
	fmt.Printf("latency  : avg %s | p50 %s | p95 %s | p99 %s | max %s\n",
		(accumulate(durations)/time.Duration(total)).Round(time.Microsecond),
		pct(0.50).Round(time.Microsecond),
		pct(0.95).Round(time.Microsecond),
		pct(0.99).Round(time.Microsecond),
		durations[total-1].Round(time.Microsecond))
}

func accumulate(ds []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum
}
