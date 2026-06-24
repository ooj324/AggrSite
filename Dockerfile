# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app
# Install gcc/musl-dev for cgo (sqlite3)
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
# Copy built frontend into the web/dist directory expected by go:embed
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build the Go app
RUN CGO_ENABLED=1 GOOS=linux go build -a -o aggrsite .

# Stage 3: Run
FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata sqlite

COPY --from=backend-builder /app/aggrsite /app/aggrsite

# Expose port
EXPOSE 4000

ENV PORT=4000
ENV HOST=0.0.0.0

CMD ["/app/aggrsite"]
