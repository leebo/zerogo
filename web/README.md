# ZeroGo Web Frontend

React + TypeScript + Ant Design web interface for ZeroGo P2P VPN mesh network.

## Tech Stack

- **React 18** - UI library
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **Ant Design 5** - UI component library
- **React Router** - Client-side routing
- **Axios** - HTTP client

## Prerequisites

- Node.js 18+
- npm or yarn

## Getting Started

### Installation

```bash
cd web
npm install
```

### Development

```bash
npm run dev
```

The development server will start at `http://localhost:3000` with API proxy to `http://localhost:8080`.

### Build

```bash
npm run build
```

Production files will be generated in `dist/` directory.

### Preview

```bash
npm run preview
```

## Project Structure

```
web/
├── src/
│   ├── api/          # API services and request config
│   ├── components/   # Reusable components
│   ├── pages/        # Page components
│   ├── types/        # TypeScript type definitions
│   ├── App.tsx       # Main app component
│   ├── main.tsx      # Application entry point
│   └── index.css     # Global styles
├── index.html        # HTML template
├── vite.config.ts    # Vite configuration
└── package.json      # Dependencies
```

## Features

- User authentication (login/register)
- Network management (CRUD operations)
- Member authorization and management
- Real-time status updates (10s polling)
- Responsive design with Ant Design

## API Integration

The frontend communicates with the ZeroGo controller backend:

- `/api/v1/auth/*` - Authentication
- `/api/v1/networks/*` - Network management
- `/api/v1/networks/:id/members/*` - Member management
- `/api/v1/peers` - Peer status

## License

MIT
