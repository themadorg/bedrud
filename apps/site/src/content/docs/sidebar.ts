import type { DocMeta } from "./meta";

export interface SidebarItem extends DocMeta {
  slug: string;
}

export interface SidebarSection {
  title: string;
  titleKey: string;
  items: SidebarItem[];
}

export const sections: SidebarSection[] = [
  {
    title: "Getting Started",
    titleKey: "gettingStarted",
    items: [
      {
        slug: "getting-started/quickstart",
        title: "Quick Start",
        description:
          "Deploy Bedrud and join a video meeting in under 5 minutes",
        order: 1,
      },

      {
        slug: "getting-started/installation",
        title: "Server Installation",
        description: "Install Bedrud server on Linux",
        order: 2,
      },
      {
        slug: "getting-started/clients",
        title: "Client Installation",
        description: "Install desktop and mobile apps",
        order: 3,
      },
      {
        slug: "getting-started/configuration",
        title: "Configuration",
        description: "Configure Bedrud for your needs",
        order: 4,
      },
      {
        slug: "getting-started/cli-installer",
        title: "CLI Installer",
        description: "One-command install for all platforms",
        order: 5,
      },
      {
        slug: "getting-started/cli-reference",
        title: "CLI Reference",
        description: "Command-line interface documentation",
        order: 6,
      },
    ],
  },
  {
    title: "Using Bedrud",
    titleKey: "using",
    items: [
      {
        slug: "using/account-and-settings",
        title: "Account & Settings",
        description:
          "Register, log in, manage profile, avatar, and preferences",
        order: 7,
      },
      {
        slug: "using/dashboard-and-rooms",
        title: "Dashboard & Rooms",
        description: "Create rooms, quick-join, and recent rooms",
        order: 8,
      },
      {
        slug: "using/joining-a-meeting",
        title: "Joining a Meeting",
        description: "Pre-join, connect, mute, camera, leave, gated rooms",
        order: 9,
      },
      {
        slug: "using/stage-mode",
        title: "Stage Mode",
        description: "Presenter layout, stage screen share, late joiners",
        order: 10,
      },
      {
        slug: "using/whiteboard",
        title: "Collaborative Whiteboard",
        description: "Shared drawing board synced over LiveKit data channels",
        order: 11,
      },
      {
        slug: "using/youtube-watch",
        title: "YouTube Watch",
        description: "Synchronized YouTube watch party in the room",
        order: 12,
      },
      {
        slug: "using/webxdc",
        title: "WebXDC mini-apps (experimental)",
        description: "Sandboxed .xdc apps in meetings — enable and use",
        order: 13,
      },
      {
        slug: "using/chat",
        title: "In-Meeting Chat",
        description: "Text, reactions, polls, and image attachments",
        order: 14,
      },
      {
        slug: "using/audio-and-push-to-talk",
        title: "Audio & Push-to-Talk",
        description: "Devices, hold-to-talk, and in-call audio controls",
        order: 15,
      },
      {
        slug: "using/meeting-controls",
        title: "Meeting Controls",
        description:
          "Screen share, spotlight, moderation, and transport fallback",
        order: 16,
      },
    ],
  },
  {
    title: "Architecture",
    titleKey: "architecture",
    items: [
      {
        slug: "architecture/overview",
        title: "Architecture Overview",
        description: "High-level system architecture of the Bedrud platform",
        order: 6,
      },
      {
        slug: "architecture/server",
        title: "Server Architecture",
        description: "Backend server design and components",
        order: 7,
      },
      {
        slug: "architecture/web",
        title: "Web Frontend",
        description: "React web application architecture",
        order: 8,
      },
      {
        slug: "architecture/android",
        title: "Android App",
        description: "Native Android application",
        order: 9,
      },
      {
        slug: "architecture/ios",
        title: "iOS App",
        description: "Native iOS application",
        order: 10,
      },
      {
        slug: "architecture/desktop",
        title: "Desktop App",
        description: "Rust-based desktop application",
        order: 11,
      },
      {
        slug: "architecture/agents",
        title: "Bot Agents",
        description: "Python bots for streaming content",
        order: 12,
      },
      {
        slug: "architecture/webrtc-connectivity",
        title: "WebRTC Connectivity",
        description: "STUN/ICE/TURN and media routing",
        order: 13,
      },
      {
        slug: "architecture/turn-server",
        title: "TURN Server",
        description: "TURN relay configuration",
        order: 14,
      },
      {
        slug: "architecture/e2ee",
        title: "End-to-End Encryption",
        description:
          "E2EE architecture, implementation, and planned client-side encryption",
        order: 15,
      },
    ],
  },
  {
    title: "Backend",
    titleKey: "backend",
    items: [
      {
        slug: "backend/overview",
        title: "Backend Documentation",
        description: "Overview of the Bedrud backend system architecture",
        order: 15,
      },
      {
        slug: "backend/structure",
        title: "Code Structure",
        description: "Backend code organization",
        order: 16,
      },
      {
        slug: "backend/database",
        title: "Database & Models",
        description: "Data models and database schema",
        order: 17,
      },
      {
        slug: "backend/authentication",
        title: "Authentication",
        description: "Auth implementation details",
        order: 18,
      },
      {
        slug: "backend/api-handlers",
        title: "API Handlers",
        description: "REST API endpoint implementations",
        order: 19,
      },
      {
        slug: "backend/livekit",
        title: "LiveKit Integration",
        description: "WebRTC media server integration",
        order: 20,
      },
      {
        slug: "backend/deployment",
        title: "Deployment",
        description: "Backend deployment strategies",
        order: 21,
      },
      {
        slug: "backend/advanced",
        title: "Advanced Topics",
        description: "Advanced backend features",
        order: 22,
      },
    ],
  },
  {
    title: "API",
    titleKey: "api",
    items: [
      {
        slug: "api/api-reference",
        title: "API Reference",
        description: "Interactive OpenAPI/Swagger API reference",
        order: 23,
      },
    ],
  },
  {
    title: "Guides",
    titleKey: "guides",
    items: [
      {
        slug: "guides/development",
        title: "Development Workflow",
        description: "Set up development environment",
        order: 27,
      },
      {
        slug: "guides/deployment",
        title: "Deployment Guide",
        description: "Deploy Bedrud to production",
        order: 28,
      },
      {
        slug: "guides/behind-proxy",
        title: "Behind a Proxy/CDN",
        description:
          "Running Bedrud behind Cloudflare, nginx, and other proxies",
        order: 29,
      },
      {
        slug: "guides/docker",
        title: "Docker Guide",
        description: "Run Bedrud with Docker",
        order: 30,
      },
      {
        slug: "guides/internal-tls",
        title: "Internal TLS",
        description: "Self-signed certificates for internal networks",
        order: 31,
      },
      {
        slug: "guides/makefile",
        title: "Makefile Reference",
        description: "Build automation commands",
        order: 32,
      },
      {
        slug: "guides/packages",
        title: "Package Installation",
        description: "Install from package managers",
        order: 33,
      },
      {
        slug: "guides/appliance",
        title: "Appliance Mode",
        description: "Pre-configured appliance setup",
        order: 34,
      },
      {
        slug: "guides/admin-dashboard",
        title: "Admin Dashboard",
        description:
          "Manage users, rooms, settings, and invite tokens from the web UI",
        order: 35,
      },
      {
        slug: "guides/roles",
        title: "User Roles",
        description:
          "Role-based access control with 5 tiers: superadmin, admin, moderator, user, guest",
        order: 36,
      },
      {
        slug: "guides/webhooks",
        title: "Webhooks",
        description: "Configure and manage webhook events",
        order: 37,
      },
      // TODO oncoming feature
      // {
      //   slug: "guides/recordings",
      //   title: "Recordings",
      //   description: "Room recording configuration and usage",
      //   order: 38,
      // },
    ],
  },
  {
    title: "Contributing",
    titleKey: "contributing",
    items: [
      {
        slug: "contributing",
        title: "Contributing",
        description: "How to contribute to Bedrud",
        order: 41,
      },
    ],
  },
];

export const sidebar = sections.flatMap((section) => section.items);

const sidebarMap = new Map<string, { item: SidebarItem; index: number }>(
  sidebar.map((item, index) => [item.slug, { item, index }]),
);

export function getPreviousDoc(slug: string): SidebarItem | undefined {
  const index = sidebarMap.get(slug)?.index;
  return index !== undefined && index > 0 ? sidebar[index - 1] : undefined;
}

export function getNextDoc(slug: string): SidebarItem | undefined {
  const index = sidebarMap.get(slug)?.index;
  return index !== undefined && index < sidebar.length - 1
    ? sidebar[index + 1]
    : undefined;
}
