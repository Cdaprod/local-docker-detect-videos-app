package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/disk"
)

// Configuration flags
var (
	jsonFile     string
	overrideDir  string
	storageMode  string
	cleanLocal   bool
	showProgress bool
)

// Video represents a video entry in the mapping file
type Video struct {
	Filename       string `json:"filename"`
	Hash           string `json:"hash"`
	UploadStatus   string `json:"upload_status"`
	UploadTimestamp string `json:"upload_timestamp,omitempty"`
}

// Mapping represents the JSON structure
type Mapping struct {
	Videos []Video `json:"videos"`
}

func init() {
	flag.StringVar(&jsonFile, "json", "video_mapping.json", "Path to the JSON mapping file")
	flag.StringVar(&overrideDir, "dir", "", "Manually specify the video directory (overrides device detection)")
	flag.StringVar(&storageMode, "storage", "local", "Storage mode: 'local' or 'icloud'")
	flag.BoolVar(&cleanLocal, "clean", false, "Delete local files after upload")
	flag.BoolVar(&showProgress, "progress", false, "Show progress using TUI (Bubbletea)")
}

// LoadMapping loads the JSON file into a Mapping struct
func LoadMapping() (Mapping, error) {
	var mapping Mapping
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		file, _ := os.Create(jsonFile)
		defer file.Close()
		json.NewEncoder(file).Encode(mapping)
	}
	file, err := os.Open(jsonFile)
	if err != nil {
		return mapping, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&mapping)
	return mapping, err
}

// SaveMapping saves the Mapping struct back to the JSON file
func SaveMapping(mapping Mapping) error {
	file, err := os.Create(jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(mapping)
}

// DetectDevice finds the first removable device (cross-platform)
func DetectDevice() (string, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return "", fmt.Errorf("error detecting devices: %v", err)
	}

	for _, partition := range partitions {
		if runtime.GOOS == "windows" {
			if strings.Contains(strings.ToLower(partition.Fstype), "removable") {
				return partition.Mountpoint, nil
			}
		} else if runtime.GOOS == "linux" {
			if strings.HasPrefix(partition.Mountpoint, "/mnt") || strings.HasPrefix(partition.Mountpoint, "/media") {
				return partition.Mountpoint, nil
			}
		}
	}

	return "", fmt.Errorf("no removable device detected")
}

// GenerateHash computes the MD5 hash of a file
func GenerateHash(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// DetectNewVideos detects videos not already in the mapping
func DetectNewVideos(mapping Mapping, videoDir string) ([]Video, error) {
	existingHashes := make(map[string]bool)
	for _, video := range mapping.Videos {
		existingHashes[video.Hash] = true
	}

	var newVideos []Video
	err := filepath.Walk(videoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isVideoFile(info.Name()) {
			hash, err := GenerateHash(path)
			if err != nil {
				return err
			}
			if !existingHashes[hash] {
				newVideos = append(newVideos, Video{
					Filename:       info.Name(),
					Hash:           hash,
					UploadStatus:   "pending",
					UploadTimestamp: "",
				})
			}
		}
		return nil
	})
	return newVideos, err
}

// isVideoFile checks if a file has a video extension
func isVideoFile(filename string) bool {
	extensions := strings.Split(".mp4,.mov,.avi,.mkv", ",")
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// ProcessLocalStorage moves video files to an archive folder
func ProcessLocalStorage(videoDir string, video Video) error {
	archiveDir := filepath.Join(videoDir, "archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		if err := os.Mkdir(archiveDir, 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %v", err)
		}
	}

	sourcePath := filepath.Join(videoDir, video.Filename)
	destPath := filepath.Join(archiveDir, video.Filename)
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move video to archive: %v", err)
	}

	fmt.Printf("Archived video: %s\n", video.Filename)
	return nil
}

// ProcessICloudStorage simulates uploading videos to iCloud
func ProcessICloudStorage(videoDir string, video Video) error {
	fmt.Printf("Uploading %s to iCloud...\n", video.Filename)
	time.Sleep(2 * time.Second) // Simulate upload delay
	if cleanLocal {
		if err := os.Remove(filepath.Join(videoDir, video.Filename)); err != nil {
			return fmt.Errorf("failed to delete local video after iCloud upload: %v", err)
		}
		fmt.Printf("Deleted local video: %s\n", video.Filename)
	}
	return nil
}

// Main Bubbletea model for progress bar
type model struct {
	total   int
	current int
	quitting bool
}

func (m model) Init() bubbletea.Cmd { return nil }

func (m model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	switch msg := msg.(type) {
	case bubbletea.KeyMsg:
		if msg.String() == "q" {
			m.quitting = true
			return m, bubbletea.Quit
		}
	case bubbletea.TickMsg:
		if m.current < m.total {
			m.current++
			return m, bubbletea.Tick(time.Second)
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	progress := float64(m.current) / float64(m.total) * 100
	return fmt.Sprintf("Progress: [%.2f%%]\nPress q to quit.", progress)
}

func main() {
	flag.Parse()

	// Determine video directory
	videoDir := overrideDir
	if videoDir == "" {
		var err error
		videoDir, err = DetectDevice()
		if err != nil {
			fmt.Printf("Error detecting device: %v\n", err)
			return
		}
	}
	fmt.Printf("Using video directory: %s\n", videoDir)

	// Load mapping
	mapping, err := LoadMapping()
	if err != nil {
		fmt.Printf("Error loading mapping: %v\n", err)
		return
	}

	// Detect new videos
	newVideos, err := DetectNewVideos(mapping, videoDir)
	if err != nil {
		fmt.Printf("Error detecting new videos: %v\n", err)
		return
	}

	// Display progress with Bubbletea if enabled
	if showProgress {
		p := bubbletea.NewProgram(model{total: len(newVideos)})
		if err := p.Start(); err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
		}
	}

	// Process each video
	for _, video := range newVideos {
		switch storageMode {
		case "local":
			err = ProcessLocalStorage(videoDir, video)
		case "icloud":
			err = ProcessICloudStorage(videoDir, video)
		default:
			fmt.Printf("Invalid storage mode: %s\n", storageMode)
			return
		}

		if err != nil {
			fmt.Printf("Failed to process video: %s (%v)\n", video.Filename, err)
			continue
		}

		// Mark video as uploaded
		video.UploadStatus = "uploaded"
		video.UploadTimestamp = time.Now().Format(time.RFC3339)
		mapping.Videos = append(mapping.Videos, video)
	}

	// Save updated mapping
	if err := SaveMapping(mapping); err != nil {
		fmt.Printf("Error saving mapping: %v\n", err)
	}
}