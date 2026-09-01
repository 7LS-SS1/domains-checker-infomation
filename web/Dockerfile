# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS dependencies
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci

FROM dependencies AS build
COPY . .
# Never let secrets leak into the build layer or the compiled client bundle —
# the only value this build ever reads is the placeholder in
# .env.local.example's shape (API_INTERNAL_URL), and that is a server-only
# env var Next.js never inlines into client code.
RUN npm run build

FROM node:22-alpine AS runtime
RUN addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app
WORKDIR /app
# Next.js "standalone" output (next.config.ts: output: "standalone") traces
# only the node_modules actually used at runtime — no full node_modules
# tree, no source, no devDependencies ship in this image.
COPY --from=build --chown=app:app /src/.next/standalone ./
COPY --from=build --chown=app:app /src/.next/static ./.next/static
# Trailing "*" makes this a no-op if public/ doesn't exist yet, rather than
# failing the build — this project currently has no static public assets.
COPY --from=build --chown=app:app /src/public* ./public
USER app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
EXPOSE 3000
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:3000/login').then(r=>process.exit(r.status<500?0:1)).catch(()=>process.exit(1))"
CMD ["node", "server.js"]
