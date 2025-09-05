# Live Broadcast Application

A monorepo containing both the frontend and backend components for a live broadcasting application.

## Repository Structure

```
/
├── frontend/       # React frontend application
├── backend/        # Go backend API
```

## Getting Started

### Prerequisites

- Node.js and npm for the frontend
- Go 1.18+ for the backend
- PostgreSQL database

### Setting Up the Environment

1. **Backend Setup**

   - Copy `.env.example` to `.env` in the backend directory
   - Update the environment variables with your configuration

   ```bash
   cd backend
   cp .env.example .env
   # Edit .env with your credentials
   ```

2. **Frontend Setup**
   - Install dependencies
   ```bash
   cd frontend
   npm install
   ```

### Running the Application

#### Backend

```bash
cd backend
go run main.go
```

#### Frontend

```bash
cd frontend
npm run dev
```

## Docker Setup

### Environment Variables

| Variable | Description | Default (docker-compose) |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@db:5432/postgres?sslmode=disable` |
| `S3_VIDEO_BUCKET` | S3 bucket for video storage | `tvstream` |
| `AWS_REGION` | AWS/MinIO region | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | S3 access key | `minio` |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key | `minio123` |
| `AWS_ENDPOINT_URL` | S3 endpoint URL | `http://minio:9000` |
| `VIDEO_DIR` | Local path for downloaded videos | `/app/videos` |
| `TEMP_DIR` | Temporary working directory | `/app/temp` |
| `LISTEN_ADDR` | Backend listen address | `:8080` |

### Starting with Docker Compose

1. Ensure Docker and Docker Compose are installed.
2. From the repository root run:
   ```bash
   docker-compose up --build
   ```
3. Access the services:
   - Frontend: http://localhost:3000
   - Backend API: http://localhost:8080
   - PostgreSQL: localhost:5432 (user `postgres`, password `postgres`)
   - MinIO console: http://localhost:9001 (user `minio`, password `minio123`)

## Contributing

1. Create a feature branch from `main`
2. Make your changes
3. Create a pull request to merge back into `main`

## Important Notes

- Never commit `.env` files or any files containing sensitive credentials
- Always use the `.env.example` files as templates with dummy values
