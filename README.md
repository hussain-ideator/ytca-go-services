# YouTube Channel Insights

A Go application that fetches public YouTube channel data using the YouTube Data API v3.

## Prerequisites

- Go 1.16 or higher
- A YouTube Data API v3 key

## Setup

1. Clone the repository:
```bash
git clone https://github.com/yourusername/yt-insights.git
cd yt-insights
```

2. Set up your environment variables:
   - Copy `.env.example` to `.env`:
     ```bash
     cp .env.example .env
     ```
   - Edit `.env` and replace `your_api_key_here` with your actual YouTube API key

## Usage

Run the application:
```bash
go run cmd/api/main.go
```

The application will fetch data for the Google Developers channel by default. You can modify the channel ID in `cmd/api/main.go` to fetch data for any other channel.

## Project Structure

```
yt-insights/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── api/
│   │   └── youtube.go
│   ├── config/
│   │   └── config.go
│   └── models/
│       └── channel.go
├── .env.example
└── README.md
```

## Features

- Fetch channel information including:
  - Channel title and description
  - Subscriber count
  - View count
  - Video count
  - Channel thumbnail

## License

MIT 