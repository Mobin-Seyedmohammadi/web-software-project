# Docker Deployment Guide

This document provides detailed instructions for building and running WASAText using Docker.

## Prerequisites

- Docker 20.10+
- Docker Compose v2.0+ (optional, for orchestration)

## Building Images

### Backend Image

Build the backend Docker image:

```bash
docker build -t wasatext-backend:latest -f Dockerfile.backend .
```

This creates a multi-stage build:
1. Builds the Go application
2. Creates a minimal Debian-based runtime image
3. Includes the compiled binary

### Frontend Image

Build the frontend Docker image:

```bash
docker build -t wasatext-frontend:latest -f Dockerfile.frontend .
```

This creates a multi-stage build:
1. Installs Node.js dependencies and builds the Vue.js app
2. Creates an nginx-based image serving the static files

## Running Containers

### Manual Container Execution

#### Backend Container

Run the backend container:

```bash
docker run -it --rm \
  -p 3000:3000 \
  -v wasatext-data:/data \
  -e PORT=3000 \
  -e DB_PATH=/data/wasatext.db \
  wasatext-backend:latest
```

Options:
- `-p 3000:3000` - Maps container port 3000 to host port 3000
- `-v wasatext-data:/data` - Persists database to a named volume
- `-e PORT=3000` - Sets the server port
- `-e DB_PATH=/data/wasatext.db` - Sets the database path

#### Frontend Container

Run the frontend container:

```bash
docker run -it --rm \
  -p 8080:80 \
  wasatext-frontend:latest
```

The frontend will be available at http://localhost:8080

### Using Docker Compose

The simplest way to run the entire application is with Docker Compose:

```bash
docker compose up
```

To run in detached mode:

```bash
docker compose up -d
```

To stop the services:

```bash
docker compose down
```

To stop and remove volumes:

```bash
docker compose down -v
```

## Docker Compose Configuration

The `docker-compose.yml` file defines two services:

### Backend Service

```yaml
backend:
  build:
    context: .
    dockerfile: Dockerfile.backend
  ports:
    - "3000:3000"
  volumes:
    - wasatext-data:/data
    - wasatext-photos:/data/photos
  environment:
    - PORT=3000
    - DB_PATH=/data/wasatext.db
  restart: unless-stopped
```

### Frontend Service

```yaml
frontend:
  build:
    context: .
    dockerfile: Dockerfile.frontend
  ports:
    - "8080:80"
  depends_on:
    backend:
      condition: service_healthy
  restart: unless-stopped
```

The frontend's `depends_on` waits for the backend's `HEALTHCHECK` to report
healthy (not just "started") before starting nginx.

## Environment Variables

### Backend

- `PORT` - HTTP server port (default: 3000)
- `DB_PATH` - Path to SQLite database file (default: /data/wasatext.db)
- `PHOTOS_DIR` - Directory for uploaded photos (default: /data/photos)

### Frontend

The frontend is configured at build time. To change the API endpoint, modify `webui/vite.config.js` before building.

## Volumes

### Data Persistence

The application uses two Docker volumes:

1. **wasatext-data**: Stores the SQLite database
2. **wasatext-photos**: Stores uploaded photos

These volumes persist data between container restarts.

To inspect volumes:

```bash
docker volume ls
docker volume inspect wasatext-data
```

## Networking

### Docker Compose Network

Docker Compose automatically creates a network for the services. Services can communicate using their service names:

- Backend is accessible at `http://backend:3000` from other containers
- Frontend is accessible at `http://frontend:80` from other containers

### Port Mapping

- **Frontend**: Host port 8080 → Container port 80
- **Backend**: Host port 3000 → Container port 3000

To use different ports, modify the `docker-compose.yml`:

```yaml
ports:
  - "8888:80"  # Frontend on port 8888
  - "3333:3000"  # Backend on port 3333
```

## Health Checks

Both images define a Docker `HEALTHCHECK`:

- **Backend**: the image also builds `cmd/healthcheck`, a small Go binary
  that calls `GET /health` on the running server and exits non-zero on
  failure. `Dockerfile.backend` runs it via:

  ```dockerfile
  HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck"]
  ```

- **Frontend**: `Dockerfile.frontend` polls nginx directly:

  ```dockerfile
  HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --spider -q http://127.0.0.1/ || exit 1
  ```

  (`127.0.0.1` rather than `localhost` avoids `wget` trying IPv6 first and
  failing, since nginx only listens on IPv4 inside the container.)

Check current health status with `docker compose ps` or
`docker inspect --format='{{json .State.Health}}' <container>`.

## Logs

View logs for all services:

```bash
docker compose logs
```

View logs for a specific service:

```bash
docker compose logs backend
docker compose logs frontend
```

Follow logs in real-time:

```bash
docker compose logs -f
```

## Scaling

Docker Compose doesn't support scaling with conflicting port mappings. To scale the backend:

1. Remove the explicit port mapping for the backend
2. Use a reverse proxy (nginx, traefik) to load balance
3. Scale the backend service:

```bash
docker compose up --scale backend=3
```

## Production Considerations

### Security

1. **Don't run as root**: The images should use a non-root user
2. **Update base images**: Regularly update base images for security patches
3. **Secrets management**: Use Docker secrets or environment variables for sensitive data
4. **Network isolation**: Use Docker networks to isolate services

### Performance

1. **Resource limits**: Set memory and CPU limits in docker-compose.yml:

```yaml
deploy:
  resources:
    limits:
      cpus: '0.5'
      memory: 512M
```

2. **Use production-ready web server**: nginx for frontend is already production-ready
3. **Database optimization**: Consider PostgreSQL for production instead of SQLite

### Monitoring

1. **Health checks**: already implemented for both services — see [Health Checks](#health-checks)
2. **Collect logs**: Use a centralized logging solution
3. **Monitor resources**: Use Docker stats or monitoring tools

## Troubleshooting

### Container won't start

Check logs:
```bash
docker compose logs backend
```

Verify the image was built correctly:
```bash
docker images | grep wasatext
```

### Frontend can't reach backend

Ensure the backend is running:
```bash
docker compose ps
```

Check that the API proxy is configured correctly in `webui/vite.config.js`

### Database errors

Ensure the data volume has correct permissions:
```bash
docker compose down
docker volume rm wasatext-data
docker compose up
```

### Port already in use

Change the port mapping in `docker-compose.yml` or stop the conflicting service.

## Cleaning Up

Remove all containers, networks, and images:

```bash
docker compose down
docker rmi wasatext-backend:latest
docker rmi wasatext-frontend:latest
```

Remove volumes (WARNING: This deletes all data):

```bash
docker compose down -v
```

Remove all unused Docker resources:

```bash
docker system prune -a
```

## Advanced Usage

### Custom Build Arguments

Pass build arguments during build:

```bash
docker build \
  --build-arg GO_VERSION=1.21 \
  -t wasatext-backend:latest \
  -f Dockerfile.backend .
```

### Multi-platform Builds

Build for multiple architectures:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t wasatext-backend:latest \
  -f Dockerfile.backend .
```

### Development with Docker

Mount source code for live reloading:

```yaml
backend:
  volumes:
    - .:/src
    - wasatext-data:/data
  command: go run ./cmd/webapi/
```

## References

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Multi-stage Builds](https://docs.docker.com/build/building/multi-stage/)
