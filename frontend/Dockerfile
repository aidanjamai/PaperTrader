# Build stage
FROM node:18-alpine AS build

WORKDIR /app

# Accept build arguments for React environment variables
ARG REACT_APP_API_URL=/api
ARG REACT_APP_GOOGLE_CLIENT_ID

# Set as environment variables for the build
ENV REACT_APP_API_URL=$REACT_APP_API_URL
ENV REACT_APP_GOOGLE_CLIENT_ID=$REACT_APP_GOOGLE_CLIENT_ID

COPY package.json package-lock.json ./
RUN npm install --legacy-peer-deps

COPY . .
RUN npm run build

# Production stage
FROM nginx:alpine

# Install wget for health checks
RUN apk add --no-cache wget

# Copy built files from build stage
COPY --from=build /app/build /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy env.sh script and make it executable
COPY env.sh /docker-entrypoint.d/env.sh
RUN chmod +x /docker-entrypoint.d/env.sh

EXPOSE 80

# Nginx 1.19+ runs scripts in /docker-entrypoint.d/ automatically before starting
CMD ["nginx", "-g", "daemon off;"]
