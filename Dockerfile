FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agoio

FROM node:24-slim AS web
WORKDIR /web
COPY package.json package-lock.json ./
RUN npm ci
COPY static ./static
RUN npx esbuild static/game.js --minify --allow-overwrite --outfile=static/game.js

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /agoio .
COPY --from=web /web/static ./static
EXPOSE 8090
CMD ["/app/agoio"]