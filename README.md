[![DigitalOcean Referral Badge](https://web-platforms.sfo2.cdn.digitaloceanspaces.com/WWW/Badge%202.svg)](https://www.digitalocean.com/?refcode=53de485c9f2c&utm_campaign=Referral_Invite&utm_medium=Referral_Program&utm_source=badge)

<a href="https://www.merge.zone"><img width="1171" height="1401" alt="image" src="https://github.com/user-attachments/assets/54e63f3b-db48-40ab-8035-49d7a74a3749" /></a>




```mermaid
architecture-beta
    group frontend(internet)[Frontend]

    service pages(internet)[HTML5 Pages] in frontend
    service serverInteraction(internet)[HTMX] in frontend
    service clientInteraction(internet)[AlpineJS] in frontend
    service diffs(internet)["@pierre/diffs"] in frontend

    group backend(server)[Backend]

    service cache(server)[go-cache] in backend
    service server(server)[Go Server] in backend
    service templ(server)[templ Components] in backend

    group api(cloud)[APIs]

    service github(cloud)[GitHub API] in api

    pages:L -- R:templ
    serverInteraction:L -- R:server
    templ:B -- T:server
    server:L -- R:cache
    cache:L -- R:github
```

## Frontend styles

Styles are built with Tailwind CSS v4 and DaisyUI. The source of truth is
`internal/views/styles/app.css` for theme tokens and runtime-only CSS; component
styling lives directly in the Templ files.

```sh
pnpm run build:css   # compile public/main.css
pnpm run watch:css   # rebuild while developing
```

`make build` regenerates Templ files and the CSS before compiling the binary.

## OpenTelemetry traces with Jaeger

`make dev` and `make watch` start a local Jaeger all-in-one container and configure Merge to export traces to it. Docker is required. Open <http://localhost:16686> to search traces. Use `make jaeger-down` to remove the container when finished.

For a non-local collector, override `OTEL_EXPORTER_OTLP_ENDPOINT`, for example:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.example.com make dev
```
