FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine AS runtime
RUN addgroup -S rootcauseway && adduser -S rootcauseway -G rootcauseway \
    && mkdir -p /var/cache/nginx /var/run \
    && chown -R rootcauseway:rootcauseway /var/cache/nginx /var/run /etc/nginx/conf.d
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
RUN chown -R rootcauseway:rootcauseway /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=3 --start-period=5s \
    CMD curl -f http://localhost:8080/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
