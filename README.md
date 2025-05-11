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

## Contributing

1. Create a feature branch from `main`
2. Make your changes
3. Create a pull request to merge back into `main`

## Important Notes

- Never commit `.env` files or any files containing sensitive credentials
- Always use the `.env.example` files as templates with dummy values
