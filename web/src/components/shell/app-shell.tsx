"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import { SidebarNav } from "@/components/shell/sidebar-nav";
import { MobileDrawer } from "@/components/shell/mobile-drawer";
import { TopBar } from "@/components/shell/top-bar";
import type { AuthUser } from "@/lib/auth/types";

interface AppShellProps {
  user: AuthUser;
  children: ReactNode;
}

export function AppShell({ user, children }: AppShellProps) {
  const [drawerOpen, setDrawerOpen] = useState(false);

  return (
    <div className="flex min-h-screen bg-background">
      <aside className="hidden w-60 shrink-0 border-r border-border bg-background lg:block">
        <SidebarNav />
      </aside>
      <MobileDrawer open={drawerOpen} onOpenChange={setDrawerOpen} />
      {/* min-w-0: a flex item's default min-width is content-based, not 0 —
          without this a wide descendant could refuse to let this column
          shrink to the viewport. */}
      <div className="flex min-h-screen min-w-0 flex-1 flex-col overflow-x-hidden">
        <TopBar user={user} onMenuClick={() => setDrawerOpen(true)} />
        <main id="main-content" className="min-w-0 flex-1 overflow-x-hidden p-4 md:p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
