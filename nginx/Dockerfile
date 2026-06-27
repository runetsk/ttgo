FROM node:22-alpine AS builder
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /var/www/ttgo
COPY nginx/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
