# Interfaz para IA local

Un cliente ligero para conversar con modelos Ollama locales. Este README resume la arquitectura, cómo ejecutar en desarrollo y producción, dónde cambiar textos/recursos (logo, notificaciones), y soluciones rápidas a problemas comunes.

---

## Resumen rápido

- Cliente: React + Vite (carpeta `client/`).
  - Entradas principales: [client/src/providers/WebSocketProvider.tsx](client/src/providers/WebSocketProvider.tsx), [client/src/service/ws.ts](client/src/service/ws.ts), [client/src/components/chat/ChatView.tsx](client/src/components/chat/ChatView.tsx).
  - Recursos públicos/origen de build: `client/public/*` (p. ej. `client/public/logo.svg`).
- Servidor: Go (carpeta `server/`).
  - Entradas principales: [server/cmd/server/main.go](server/cmd/server/main.go), [server/internal/ws/handler.go](server/internal/ws/handler.go), [server/internal/ollama/client.go](server/internal/ollama/client.go).
  - Sirve archivos estáticos compilados desde `server/cmd/server/static` (ruta usada en `main.go`).
- Comunicación: WebSocket entre cliente y servidor para streaming de respuestas y "thinking chunks".

---

## Requisitos previos

- Node.js (v16+ recomendado) y npm
- Go (1.20+ recomendado)
- Ollama corriendo localmente (según la configuración en `server/internal/ollama/*`)
- PowerShell (si trabajas en Windows, se muestran ejemplos en PowerShell)

---

## Modo desarrollo (hot-reload)

1. Abrir terminal para el cliente:
```powershell
cd 'c:\Users\Emmanuel\Downloads\Catbot-de-IA-local-main\Catbot-de-IA-local-main\client'
npm install
npm run dev
# cliente en: http://localhost:5173
```

2. Abrir terminal para el servidor (Go):
```powershell
cd 'c:\Users\Emmanuel\Downloads\Catbot-de-IA-local-main\Catbot-de-IA-local-main\server\cmd\server'
# instalar dependencias Go si faltan, por ejemplo:
go get github.com/gorilla/handlers@v1.5.1
go mod tidy
go run .
# servidor en: http://localhost:8080 (API / WebSocket)
```

- En desarrollo normalmente abres la UI en `http://localhost:5173` (Vite). El servidor solo provee API/WS en el puerto 8080.

---

## Build para producción (servido por Go)

Cuando quieras que Go sirva la UI (archivo estático):

1. Generar la build del cliente (Vite está configurado para escribir la salida en `server/cmd/server/static`):
```powershell
cd 'c:\Users\Emmanuel\Downloads\tiny-ollama-chat-master\tiny-ollama-chat-master\client'
npm install
npm run build
```

2. Reiniciar el servidor Go desde la carpeta con `static`:
```powershell
cd 'c:\Users\Emmanuel\Downloads\tiny-ollama-chat-master\tiny-ollama-chat-master\server\cmd\server'
go run .
# verá linea "Serving static files from: <ruta>" en la salida
```

- Si ves la UI antigua al abrir `http://localhost:8080`, asegúrate de:
  - Que `npm run build` haya escrito en `server/cmd/server/static`.
  - Reiniciar el servidor Go desde `server/cmd/server` (porque `main.go` usa `os.Getwd()`).
  - Limpiar cache del navegador (Ctrl+F5 o ventana incógnito).

---

## Dónde están los textos / traducciones (UI y notificaciones)

Editar literales en el cliente es seguro; solo rebuild/reload para producción.

- Notificaciones y toasts: [client/src/providers/WebSocketProvider.tsx](client/src/providers/WebSocketProvider.tsx)
- Cabeceras y textos del chat: [client/src/components/chat/ChatView.tsx](client/src/components/chat/ChatView.tsx)
- Barra lateral / botones: [client/src/components/chat/Sidebar.tsx](client/src/components/chat/Sidebar.tsx)
- Páginas: [client/src/pages/Chat.tsx](client/src/pages/Chat.tsx)

Pasos para cambiar textos:
1. Editar archivos en `client/src`.
2. En dev: `npm run dev` mostrará cambios inmediatamente.
3. En prod: `npm run build` y reiniciar Go.

---

## Reemplazar logo / favicon

- Origen (dev): `client/public/logo.svg`, `client/public/logo.png`
- Build (prod servido por Go): `server/cmd/server/static/logo.svg`, `server/cmd/server/static/logo.png`

Reemplazo rápido:
```powershell
# copiar nuevo logo al origen (recomendado)
Copy-Item 'C:\ruta\a\nuevo-logo.svg' 'c:\...tiny-ollama-chat-master\client\public\logo.svg' -Force
# build y reiniciar servidor Go
cd 'c:\...tiny-ollama-chat-master\client'; npm run build
cd 'c:\...tiny-ollama-chat-master\server\cmd\server'; go run .
```

O copia directo a `server/cmd/server/static` para pruebas inmediatas y luego Ctrl+F5.

---

## Integración Ollama

- Cliente llama al servidor vía WS (eventos `start_conversation`, `message`, `resume_conversation`).
  - Implementación cliente WS: [client/src/service/ws.ts](client/src/service/ws.ts)
- Servidor invoca Ollama y stream de respuestas ocurre en:
  - [server/internal/ws/handler.go](server/internal/ws/handler.go)
  - Cliente Ollama: [server/internal/ollama/client.go](server/internal/ollama/client.go)

Asegúrate Ollama esté corriendo en la dirección que `server/internal/config/config.go` (o variables de entorno que use) espera.

---

## Problemas comunes y soluciones rápidas

- npm run build falla con ENOENT: "Could not read package.json"
  - Causa: estás en la carpeta raíz; ejecuta `npm run build` desde `client/` o usa:
    ```powershell
    npm --prefix 'c:\...tiny-ollama-chat-master\client' run build
    ```
- CORS preflight blocked (peticiones desde 5173 a 8080)
  - Asegúrate de que el servidor Go tenga middleware CORS (p. ej. `github.com/gorilla/handlers`) y permita `http://localhost:5173`. Ejemplo en `server/cmd/server/main.go`.
  - Si falta el módulo: desde `server/cmd/server` ejecuta:
    ```powershell
    go get github.com/gorilla/handlers@v1.5.1
    go mod tidy
    ```
- Versión antigua mostrada al borrar `static`
  - Verifica qué puerto estás abriendo (5173 → Vite; 8080 → Go).
  - `netstat -ano | Select-String ":5173|:8080"` muestra conectividad; si ves `SYN_SENT` es porque no hay servidor escuchando.
  - Identifica procesos por PID con:
    ```powershell
    Get-Process -Id <PID>
    Get-CimInstance Win32_Process -Filter "ProcessId = <PID>" | Select CommandLine
    ```
- `go run .` da error por módulo faltante: ejecutar `go get ...` y `go mod tidy` en la carpeta del servidor.
- Si los assets no concuerdan, revisa `client/vite.config.ts` (outDir) y el `static` que `main.go` declara.

---

## Scripts útiles (PowerShell)

- Levantar cliente (dev):
```powershell
cd 'c:\...tiny-ollama-chat-master\client'
npm install
npm run dev
```

- Build cliente y servir con Go:
```powershell
cd 'c:\...tiny-ollama-chat-master\client'
npm install
npm run build

cd 'c:\...tiny-ollama-chat-master\server\cmd\server'
go get github.com/gorilla/handlers@v1.5.1
go mod tidy
go run .
```

- Forzar limpieza de cache en navegador: abrir en ventana incógnito o Ctrl+F5.

---

## Archivos clave (rápida referencia)

- Servidor:
  - [server/cmd/server/main.go](server/cmd/server/main.go)
  - [server/internal/ws/handler.go](server/internal/ws/handler.go)
  - [server/internal/ollama/client.go](server/internal/ollama/client.go)
  - [server/internal/database/db.go](server/internal/database/db.go)
- Cliente:
  - [client/src/providers/WebSocketProvider.tsx](client/src/providers/WebSocketProvider.tsx)
  - [client/src/service/ws.ts](client/src/service/ws.ts)
  - [client/src/components/chat/ChatView.tsx](client/src/components/chat/ChatView.tsx)
  - [client/public/logo.svg](client/public/logo.svg)
  - [client/vite.config.ts](client/vite.config.ts)

---

## Consejos para desarrollo ágil

- Usa `npm run dev` (Vite) para trabajar en UI y ver cambios en caliente en `http://localhost:5173`.
- Para probar integración real con el servidor y Ollama, mantén servidor Go corriendo y usa Vite (las llamadas API/WS apuntan a `http://localhost:8080`).
- Si prefieres abrir una sola URL `http://localhost:8080` y recibir hot-reload, añade en `server/cmd/server/main.go` un modo DEV que proxyee estáticos a `http://localhost:5173` (puedo ayudarte a añadirlo).

---

## Contribuciones y mantenimiento

- Para cambios en literales UI: editar `client/src/...`, luego `npm run build` para producción.
- Para cambios en la integración con Ollama o DB: revisar y testear en `server/internal/ollama` y `server/internal/database`.

---
