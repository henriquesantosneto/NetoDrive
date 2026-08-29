package syncer

import (
	"fmt"
	"time"
)

func scanLocalFilesWithTimeout(localRoot string, timeout time.Duration) (map[string]string, error) {
	type result struct {
		local map[string]string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		local, err := scanLocalFiles(localRoot)
		ch <- result{local, err}
	}()
	select {
	case r := <-ch:
		return r.local, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("scan da pasta local excedeu %s (Explorer ou pasta muito grande)", timeout)
	}
}

func scanLocalFilesLightWithTimeout(localRoot string, known map[string]string, timeout time.Duration) (map[string]string, error) {
	type result struct {
		local map[string]string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		local, err := scanLocalFilesLight(localRoot, known)
		ch <- result{local, err}
	}()
	select {
	case r := <-ch:
		return r.local, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("scan leve excedeu %s (CFAPI/Explorer)", timeout)
	}
}
