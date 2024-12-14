# Golang Detect Videos on Local Devices 

Featuring… 
GoPSUtils, 
Bubbletea, 
Docker, 
and cameos staring: Linux, Windows, and Nikon!

Features
1. CLI Flags:
- -json: Path to the JSON mapping file.
- -dir: Override the detected video directory.
- -progress: Display progress using Bubbletea.
2. Bubbletea Integration:
- Shows a progress bar for detected videos.
- Graceful handling of q to quit.
3. Cross-Platform:
- Combines gopsutil for device detection and platform-specific logic for paths.
	