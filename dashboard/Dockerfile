FROM node:20-alpine AS frontend
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -o dashboard .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=backend /build/dashboard /usr/local/bin/dashboard
EXPOSE 8090
CMD ["dashboard"]
