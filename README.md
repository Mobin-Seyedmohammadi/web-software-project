# WASAText

WASAText is a modern messaging platform that enables users to communicate through text and photo messages in both private conversations and group chats.

This project was developed for the [Web and Software Architecture](http://gamificationlab.uniroma1.it/en/wasa/) course @ Sapienza University of Rome.

## Features

- **Private Conversations**: One-on-one messaging with other users
- **Group Chats**: Create and manage group conversations
- **Text & Photo Messages**: Send text messages and photos
- **Message Reactions**: React to messages with emoji
- **Message Forwarding**: Forward messages between conversations
- **User Profiles**: Customize username and profile photo
- **Real-time Updates**: See conversation list sorted by latest messages

## Project Structure

* `cmd/` contains all executables
	* `cmd/healthcheck` - Health check daemon for server monitoring
	* `cmd/webapi` - Main web API server
* `demo/` contains demo configuration files
* `doc/` contains the OpenAPI API specification
* `service/` contains all business logic packages
	* `service/api` - HTTP API handlers
	* `service/db` - Database layer with SQLite
* `vendor/` contains Go dependencies (managed by Go modules)
* `webui/` contains the Vue.js frontend application

## Prerequisites

- **Go 1.21+** for backend development
- **Node.js 20+** for frontend development  
- **Docker & Docker Compose** for containerized deployment (optional)
- **SQLite3** for database (included in Go driver)

## Getting Started

### Development Mode

#### Backend

To run the backend API server:

```bash
go run ./cmd/webapi/
```

The server will start on port 3000 by default. You can customize the port:

```bash
PORT=8080 go run ./cmd/webapi/
```

#### Frontend

To run the frontend development server:

```bash
./open-node.sh
# Inside the container:
yarn run dev
```

The frontend will be available at http://localhost:5173

### Production Build

#### Build Frontend for Production

```bash
./open-node.sh
# Inside the container:
yarn run build-prod
exit
```

#### Build Backend with Embedded Frontend

```bash
go build -tags webui ./cmd/webapi/
```

This creates a single executable with the frontend embedded.

## API Documentation

The API is documented using OpenAPI 3.0 specification. See `doc/api.yaml` for the complete API documentation.

### Key Endpoints

- `POST /session` - User login/registration (simplified auth)
- `GET /conversations` - List all conversations
- `GET /conversations/{id}` - Get conversation details
- `POST /conversations/{id}/messages` - Send a message
- `POST /groups` - Create a new group
- `PUT /users/me/username` - Update username
- `PUT /users/me/photo` - Upload profile photo

All endpoints (except `/session`) require Bearer authentication using the user identifier from login.

## Docker Deployment

### Build Docker Images

Build the backend image:

```bash
docker build -t wasatext-backend:latest -f Dockerfile.backend .
```

Build the frontend image:

```bash
docker build -t wasatext-frontend:latest -f Dockerfile.frontend .
```

### Run with Docker Compose

From the project root:

```bash
docker compose up --build
```

This builds both images (if needed) and starts the backend and frontend:
- **Frontend**: http://localhost:8080
- **Backend API**: http://localhost:3000

Both services define a Docker `HEALTHCHECK` (backend polls its own `GET
/health`, frontend polls nginx). The frontend won't start until the backend
reports healthy, so the first `docker compose up` may take a few seconds
before both containers show as `healthy` — check with:

```bash
docker compose ps
```

Data persists across restarts in two named volumes: `wasatext-data` (the
SQLite database) and `wasatext-photos` (uploaded profile/group photos).

To stop the stack:

```bash
docker compose down          # keep data
docker compose down -v       # also wipe the named volumes
```

To run in the background, add `-d` to the `up` command.

See [DOCKER.md](DOCKER.md) for detailed Docker instructions.

## Environment Variables

- `PORT` - Server port (default: 3000)
- `DB_PATH` - SQLite database file path (default: ./wasatext.db)
- `PHOTOS_DIR` - Directory for uploaded photos (default: ./photos)
- `SERVER_URL` - Base URL for health checks (default: http://localhost:3000)

## Database

The application uses SQLite for data storage. The database schema is automatically initialized on first run.

### Database Schema

- **users** - User accounts and profiles
- **conversations** - Private and group conversations
- **conversation_participants** - Conversation membership
- **messages** - Text and photo messages
- **message_delivery** - Message delivery status tracking
- **reactions** - Emoji reactions to messages

## Go Vendoring

This project uses [Go Vendoring](https://go.dev/ref/mod#vendoring). After modifying dependencies:

```bash
go mod tidy
go mod vendor
```

Commit all files under `vendor/` directory.

## Frontend Development

The frontend is built with:
- **Vue.js 3** - Progressive JavaScript framework
- **Vue Router** - Official router for Vue.js
- **Vite** - Next generation frontend tooling

### Project Scripts

- `yarn run dev` - Start development server
- `yarn run build-prod` - Build for production
- `yarn run build-embed` - Build for embedding in Go binary
- `yarn run preview` - Preview production build

## Testing Production Build

To test the production build locally:

**Terminal 1 - Start backend:**
```bash
go run ./cmd/webapi/
```

**Terminal 2 - Build and preview frontend:**
```bash
cd webui
yarn run build-prod
yarn run preview
```

Then open http://localhost:4173

## Known Issues

### JavaScript Errors in Production

Some errors may not appear in `vite` development mode. Always test with:

```bash
cd webui
yarn run build-prod
yarn run preview
```

## Authentication

WASAText uses simplified authentication for educational purposes:

- Users log in with just a username (no password)
- If the username exists, the user is logged in
- If the username is new, a new account is created
- The API returns a user identifier used as a Bearer token

In production, you would integrate with a proper identity provider.

## License

This project is developed for educational purposes as part of the Web and Software Architecture course @ Sapienza University of Rome.

## Development

### Code Style

- **Go**: Follow standard Go formatting (use `gofmt`)
- **JavaScript/Vue**: 2-space indentation
- **Configuration files**: See `.editorconfig` for details

### Adding New Features

1. Update OpenAPI specification in `doc/api.yaml`
2. Implement database operations in `service/db/`
3. Create API handlers in `service/api/`
4. Update frontend components in `webui/src/`
5. Test thoroughly in both development and production modes

## Support

For questions related to the course project, please refer to the course materials or contact the teaching staff.

## Acknowledgments

- Course: Web and Software Architecture @ Sapienza University of Rome
- Template based on: [Fantastic Coffee (Decaffeinated)](https://github.com/sapienzaapps/fantastic-coffee-decaffeinated)
