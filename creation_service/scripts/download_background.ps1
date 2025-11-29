# Download YouTube video as background
# Usage: .\download_background.ps1

$videoUrl = "https://www.youtube.com/watch?v=U-YGuQlVzf8"
$outputDir = "backgroundvids"

# Check if yt-dlp is installed
if (!(Get-Command yt-dlp -ErrorAction SilentlyContinue)) {
    Write-Host "Installing yt-dlp..."
    pip install yt-dlp
}

# Download video
Write-Host "Downloading video to $outputDir..."
yt-dlp -f "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best" -o "$outputDir/%(title)s.mp4" $videoUrl

Write-Host "Download complete!"
