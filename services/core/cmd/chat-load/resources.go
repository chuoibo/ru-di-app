package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RSS samples are per-process, not whole-machine or PostgreSQL measurements.
func processRSS(pid int) int64 {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		parts := strings.Fields(scan.Text())
		if len(parts) >= 2 && parts[0] == "VmRSS:" {
			n, err := strconv.ParseInt(parts[1], 10, 64)
			if err == nil {
				return n
			}
		}
	}
	return -1
}
