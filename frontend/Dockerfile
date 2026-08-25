FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine AS runtime
# COPY before the user/perm RUN below -- with the legacy builder (no buildx
# on the homelab-targeting build host) cross-building this multi-arch base
# image, a COPY immediately after a RUN fails with "found but does not
# provide the specified platform", even after re-pulling the base image for
# that platform. Doing the COPYs first and chown-ing everything in one RUN
# afterward sidesteps it with no behavior change.
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
RUN addgroup -S rootcauseway && adduser -S rootcauseway -G rootcauseway \
    && mkdir -p /var/cache/nginx /var/run \
    && chown -R rootcauseway:rootcauseway /var/cache/nginx /var/run /etc/nginx/conf.d /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=3 --start-period=5s \
    CMD curl -f http://localhost:8080/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
