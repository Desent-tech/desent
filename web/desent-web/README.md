# desent web


<details>
<summary>Русская версия</summary>

# desent web

Веб-фронтенд для [desent](../../README.md) — децентрализованной стриминговой платформы.

Построен на Next.js 16, React 19, shadcn/ui и Tailwind CSS 4 в монорепозитории Turborepo.

## Быстрый старт

Требуется Node.js 20+ и pnpm 9+.

```bash
# 1. Установить зависимости
pnpm install

# 2. Запустить dev-сервер
pnpm dev
```

Приложение будет доступно по адресу `http://localhost:3000`.

## Сборка и продакшен

```bash
# Собрать для продакшена
pnpm build

# Запустить продакшен-сервер
pnpm --filter web start
```

### Docker

```bash
docker build -t desent-web .
docker run -p 3000:3000 desent-web
```

Или используйте `docker compose up` из корня проекта, чтобы запустить Go-сервер и веб-фронтенд вместе.

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `NEXT_PUBLIC_BACKEND_URL` | — | URL Go-сервера desent (например, `http://localhost:8080`) |

## Доступные команды

| Команда | Описание |
|---|---|
| `pnpm dev` | Запуск dev-сервера с Turbopack |
| `pnpm build` | Продакшен-сборка |
| `pnpm lint` | Запуск ESLint |
| `pnpm format` | Форматирование кода через Prettier |
| `pnpm typecheck` | Проверка типов TypeScript |

## Структура проекта

```
desent/
├── apps/
│   └── web/                     # Приложение Next.js
│       ├── app/                 # Страницы App Router
│       │   ├── admin/           # Панель администратора (настройки, пользователи)
│       │   ├── auth/            # Вход и регистрация
│       │   ├── profile/         # Профиль пользователя
│       │   └── vods/            # Архив записей
│       ├── components/          # Компоненты приложения
│       └── hooks/               # Пользовательские React-хуки
├── packages/
│   ├── ui/                      # Общие UI-компоненты (shadcn/ui)
│   ├── eslint-config/           # Общая конфигурация ESLint
│   └── typescript-config/       # Общая конфигурация TypeScript
├── turbo.json                   # Конфигурация пайплайна Turborepo
├── Dockerfile                   # Многоэтапная продакшен-сборка
└── package.json                 # Корень воркспейса
```

## Добавление компонентов shadcn/ui

```bash
pnpm dlx shadcn@latest add <component> -c apps/web
```

Компоненты размещаются в `packages/ui/src/components/` и импортируются так:

```tsx
import { Button } from "@workspace/ui/components/button";
```

</details>

Web frontend for [desent](../../README.md) — a decentralized streaming platform.

Built with Next.js 16, React 19, shadcn/ui, and Tailwind CSS 4 in a Turborepo monorepo.


## Quick Start

Requires Node.js 20+ and pnpm 9+.

```bash
# 1. Install dependencies
pnpm install

# 2. Start dev server
pnpm dev
```

The app will be available at `http://localhost:3000`.

## Build & Production

```bash
# Build for production
pnpm build

# Start production server
pnpm --filter web start
```

### Docker

```bash
docker build -t desent-web .
docker run -p 3000:3000 desent-web
```

Or use `docker compose up` from the project root to run both the Go server and the web frontend together.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_BACKEND_URL` | — | URL of the desent Go server (e.g., `http://localhost:8080`) |

## Available Scripts

| Command | Description |
|---|---|
| `pnpm dev` | Start dev server with Turbopack |
| `pnpm build` | Production build |
| `pnpm lint` | Run ESLint |
| `pnpm format` | Format code with Prettier |
| `pnpm typecheck` | Run TypeScript type checking |

## Project Structure

```
desent/
├── apps/
│   └── web/                     # Next.js application
│       ├── app/                 # App Router pages
│       │   ├── admin/           # Admin panel (settings, users)
│       │   ├── auth/            # Login & registration
│       │   ├── profile/         # User profile
│       │   └── vods/            # VOD archive
│       ├── components/          # App-specific components
│       └── hooks/               # Custom React hooks
├── packages/
│   ├── ui/                      # Shared UI components (shadcn/ui)
│   ├── eslint-config/           # Shared ESLint configuration
│   └── typescript-config/       # Shared TypeScript configuration
├── turbo.json                   # Turborepo pipeline config
├── Dockerfile                   # Multi-stage production build
└── package.json                 # Workspace root
```

## Adding shadcn/ui Components

```bash
pnpm dlx shadcn@latest add <component> -c apps/web
```

Components are placed in `packages/ui/src/components/` and imported as:

```tsx
import { Button } from "@workspace/ui/components/button";
```

---
