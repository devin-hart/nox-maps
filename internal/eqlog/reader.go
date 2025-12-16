package eqlog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type LogLine struct {
	Line string
	Time time.Time
}

type Reader struct {
	EqDir       string
	Lines       chan LogLine
	InitialZone string
}

func NewReader(eqDir string) *Reader {
	return &Reader{
		EqDir: eqDir,
		Lines: make(chan LogLine, 1000),
	}
}

func (r *Reader) Start() error {
	// Try to detect initial zone from log history
	r.detectInitialZone()
	go r.pollAndRead()
	return nil
}

func (r *Reader) detectInitialZone() {
	logPath, err := r.findLatestLog()
	if err != nil {
		return
	}

	file, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer file.Close()

	// Read last 50KB of log to find most recent zone
	stat, _ := file.Stat()
	startPos := stat.Size() - 50000
	if startPos < 0 {
		startPos = 0
	}
	file.Seek(startPos, 0)

	scanner := bufio.NewScanner(file)
	zoneRegex := regexp.MustCompile(`You have entered (.+)\.`)

	var lastZone string
	for scanner.Scan() {
		line := scanner.Text()
		if matches := zoneRegex.FindStringSubmatch(line); len(matches) == 2 {
			lastZone = matches[1]
		}
	}

	if lastZone != "" {
		r.InitialZone = lastZone
		fmt.Printf("🌍 Detected initial zone from log: '%s'\n", lastZone)
	}
}

func (r *Reader) pollAndRead() {
	var currentPath string
	var file *os.File
	var reader *bufio.Reader
	
	// Check for new files every 3 seconds
	checkInterval := 3 * time.Second
	lastCheck := time.Now()

	for {
		// 1. Check for Character Switch
		if time.Since(lastCheck) > checkInterval {
			latestPath, err := r.findLatestLog()
			if err == nil && latestPath != currentPath {
				fmt.Printf("🔄 Loading Log: %s\n", filepath.Base(latestPath))
				
				if file != nil {
					file.Close()
				}

				newFile, err := os.Open(latestPath)
				if err != nil {
					fmt.Printf("❌ Error opening log: %v\n", err)
				} else {
					// SMART SEEK:
					// Instead of skipping to the very end (SeekEnd), back up 5KB.
					// This ensures we catch the "You have entered..." message 
					// that often appears right before/during login.
					stat, _ := newFile.Stat()
					startPos := stat.Size() - 5000 
					if startPos < 0 { startPos = 0 }
					newFile.Seek(startPos, 0)
					
					file = newFile
					currentPath = latestPath
					reader = bufio.NewReader(file)
				}
			}
			lastCheck = time.Now()
		}

		// 2. Read Loop
		if reader != nil {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			cleanLine := strings.TrimSpace(line)
			if cleanLine != "" {
				r.Lines <- LogLine{
					Line: cleanLine,
					Time: time.Now(),
				}
			}
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

func (r *Reader) findLatestLog() (string, error) {
	// Check Root
	logs, _ := r.scanDir(r.EqDir)
	
	// Check Logs Subdir
	if len(logs) == 0 {
		subDir := filepath.Join(r.EqDir, "Logs")
		logs, _ = r.scanDir(subDir)
	}

	if len(logs) == 0 {
		return "", fmt.Errorf("no logs found")
	}

	// Sort Oldest -> Newest
	sort.Slice(logs, func(i, j int) bool {
		fi, _ := os.Stat(logs[i])
		fj, _ := os.Stat(logs[j])
		return fi.ModTime().Before(fj.ModTime())
	})

	return logs[len(logs)-1], nil
}

func (r *Reader) scanDir(path string) ([]string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "eqlog") && strings.HasSuffix(f.Name(), ".txt") {
			logs = append(logs, filepath.Join(path, f.Name()))
		}
	}
	return logs, nil
}