# Swami Samarth Marg Project

A comprehensive WhatsApp-based seva management system for religious organizations, featuring automated group management, poll-based seva scheduling, and an admin portal for oversight.

## Project URL : https://caring-truth-production.up.railway.app/

## Overview

This project consists of two main components:

1. **WhatsApp Bridge** - A Go-based WhatsApp client that manages group communications, polls, and member data
2. **Admin Portal** - A Next.js web application for managing sevas, groups, and members

## Features

### WhatsApp Bridge
- 📱 **WhatsApp Integration** - Full WhatsApp Web API integration using whatsmeow
- 🗳️ **Poll Management** - Create and track polls for seva scheduling
- 👥 **Group Management** - Automated member import and group organization
- 💾 **Data Persistence** - SQLite for local storage, PostgreSQL support for production
- 🔐 **API Security** - Bearer token authentication for all endpoints
- 📊 **LID-Phone Mapping** - Track WhatsApp LID to phone number mappings
- 🖼️ **QR Code Authentication** - Multiple QR code endpoints for easy login

### Admin Portal
- 🎯 **Seva Operations** - Manage religious services and schedules
- 👥 **Group Management** - Organize and oversee WhatsApp groups
- 📇 **Member Directory** - Track participant information
- 🎨 **Modern UI** - Built with Next.js, React, and TailwindCSS
- 🔒 **Session Management** - Secure admin authentication

## Architecture

```
SwamiSamarthMargProject/
├── whatsapp-bridge/        # Go backend service
│   ├── main.go            # Main application entry
│   ├── admin_api.go       # Admin API endpoints
│   ├── integration.go     # WhatsApp integration logic
│   ├── import_members.go  # Member import utilities
│   ├── domain/            # Domain models
│   ├── handler/           # HTTP handlers
│   ├── repository/        # Data access layer
│   ├── service/           # Business logic
│   ├── util/              # Utility functions
│   └── config/            # Configuration files
│
├── admin-portal/          # Next.js frontend
│   ├── app/              # Next.js app directory
│   │   ├── dashboard/    # Dashboard pages
│   │   ├── groups/       # Group management
│   │   ├── members/      # Member directory
│   │   └── api/          # API routes
│   ├── components/       # React components
│   └── lib/              # Utilities and helpers
│
└── store/                # Local data storage
    ├── messages.db       # Message history (SQLite)
    └── whatsapp.db       # WhatsApp session data
```

## Prerequisites

### WhatsApp Bridge
- Go 1.24.1 or higher
- SQLite (included)
- PostgreSQL (optional, for production)

### Admin Portal
- Node.js 20.x or higher
- npm or yarn

## Installation

### 1. WhatsApp Bridge Setup

```bash
cd whatsapp-bridge

# Install dependencies
go mod download

# Build the application
go build -o app

# Set up environment variables
cp .env.example .env
# Edit .env with your configuration
```

#### Required Environment Variables

```bash
# API Security
API_KEY=your-secret-api-key

# Admin Authentication
ADMIN_PASSWORD_HASH=your-bcrypt-password-hash
ADMIN_SESSION_SECRET=your-session-secret

# Database (Optional - for production)
POSTGRES_DSN=postgresql://user:password@host:port/database

# Server Configuration
PORT=8081
IMPORT_MEMBERS=1  # Set to 1 to import members on startup
```

### 2. Admin Portal Setup

```bash
cd admin-portal

# Install dependencies
npm install

# Set up environment variables
cp .env.example .env.local
# Edit .env.local with your configuration

# Development mode
npm run dev

# Production build
npm run build
npm start
```

## Usage

### Starting the WhatsApp Bridge

```bash
cd whatsapp-bridge
./start.sh
```

Or manually:
```bash
export PORT=8081
export API_KEY=your-api-key
export ADMIN_PASSWORD_HASH=your-hash
export ADMIN_SESSION_SECRET=your-secret
./app
```

### WhatsApp Authentication

1. Start the bridge service
2. Access the QR code endpoint: `http://localhost:8081/login`
3. Scan the QR code with WhatsApp mobile app
4. The service will automatically connect

### API Endpoints

#### Health & Status
- `GET /healthz` - Health check
- `GET /status` - Connection status and QR code info
- `GET /` - Service information

#### QR Code & Authentication
- `GET /login` - Get QR code for login (PNG image)
- `GET /login-android` - Android-optimized QR code
- `GET /qr-generate` - Generate new QR code
- `GET /qr-terminal` - Terminal-formatted QR code
- `GET /qr-raw` - Raw QR code string
- `GET /qr-debug` - QR code debug information

#### Messaging (Requires API Key)
- `POST /send-message` - Send WhatsApp message
  ```json
  {
    "recipient": "919876543210",
    "message": "Your message",
    "media_path": "/path/to/file" // optional
  }
  ```

- `POST /send-poll` - Create and send poll
  ```json
  {
    "recipient": "919876543210@g.us",
    "poll_name": "Select your preferred date",
    "poll_options": ["Option 1", "Option 2", "Option 3"],
    "selectable_count": 1
  }
  ```

### Starting the Admin Portal

```bash
cd admin-portal
npm run dev
```

Access at: `http://localhost:3000`

## Group Management

The system supports multiple seva groups organized in CSV files:

- **Darbar** - Durga Paath groups (11 groups)
- **EkadashiBhagavatSeva** - Ekadashi Bhagavat Seva groups (10 groups)
- **Malhari** - Malhari groups (9 groups)
- **Navratri** - Navratri Paath groups (8 groups)
- **SaptahikSwamiSeva** - Weekly Swami Seva groups (19 groups)

CSV files contain member information and are automatically imported on startup.

## Database Schema

### Messages Database (SQLite/PostgreSQL)

**chats**
- `jid` (PRIMARY KEY) - Chat identifier
- `name` - Chat name
- `last_message_time` - Last message timestamp

**messages**
- `id` - Message ID
- `chat_jid` - Chat reference
- `sender` - Sender identifier
- `content` - Message content
- `timestamp` - Message timestamp
- `is_from_me` - Boolean flag
- `media_type` - Type of media
- `filename` - Media filename
- `url` - Media URL
- Media metadata fields

**lid_phone_mapping**
- `lid` (PRIMARY KEY) - WhatsApp LID
- `phone_number` - Associated phone number
- `group_jid` - Group identifier
- `timestamp` - Mapping timestamp
- `source` - Source of mapping

**poll_data**
- `poll_id` (PRIMARY KEY) - Poll identifier
- `poll_name` - Poll question
- `chat_jid` - Chat where poll was sent
- `poll_options` - JSON array of options
- `timestamp` - Creation timestamp

**poll_votes**
- `poll_id` - Poll reference
- `voter_jid` - Voter identifier
- `voted_option_names` - JSON array of selected options
- `timestamp` - Vote timestamp

## Security

- All API endpoints (except health checks and QR codes) require Bearer token authentication
- Admin portal uses session-based authentication with bcrypt password hashing
- Environment variables for sensitive configuration
- CORS support for cross-origin requests

## Deployment

### Railway / Cloud Deployment

The project is configured for Railway deployment:

1. Set environment variables in Railway dashboard
2. Railway automatically detects `start.sh` and runs it
3. The `PORT` variable is automatically provided by Railway

Required Railway Variables:
- `API_KEY`
- `ADMIN_PASSWORD_HASH`
- `ADMIN_SESSION_SECRET`
- `POSTGRES_DSN` (optional)

## Development

### Building from Source

```bash
# WhatsApp Bridge
cd whatsapp-bridge
go build -o app

# Admin Portal
cd admin-portal
npm run build
```

### Running Tests

```bash
# Go tests
cd whatsapp-bridge
go test ./...

# Frontend tests
cd admin-portal
npm test
```

## Technologies Used

### Backend
- **Go 1.24.1** - Primary language
- **whatsmeow** - WhatsApp Web API library
- **SQLite** - Local database
- **PostgreSQL** - Production database (optional)
- **Protocol Buffers** - Message serialization

### Frontend
- **Next.js 14** - React framework
- **React 18** - UI library
- **TypeScript** - Type safety
- **TailwindCSS** - Styling
- **Lucide React** - Icons

## Screenshots

![Admin Portal Seva Dashboard](seva-dashboard.png)
![Whatsapp Group Configuration](group-config.png)
![Whatsapp Members Configuration](members-dasboard.png)

## Troubleshooting

### QR Code Not Displaying
- Check if the service is running: `curl http://localhost:8081/status`
- Try regenerating: `curl http://localhost:8081/qr-generate`
- Check logs for connection errors

### Database Connection Issues
- Verify `POSTGRES_DSN` format if using PostgreSQL
- Check file permissions for SQLite databases in `store/` directory
- Ensure `store/` directory exists

### API Authentication Failures
- Verify `API_KEY` environment variable is set
- Include header: `Authorization: Bearer your-api-key`
- Check for typos in the API key


## Contributing

This is a religious community project. Contributions should maintain the spiritual and organizational focus of the application.

## License

This project is maintained for the Swami Samarth Marg community.

## Support

For issues or questions, please contact the project maintainers.
