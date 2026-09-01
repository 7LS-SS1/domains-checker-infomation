"use client";

import { useMutation } from "@tanstack/react-query";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { LogOut, User } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { bffFetch } from "@/lib/api/client";
import { logoutDataSchema } from "@/lib/auth/types";
import type { AuthUser } from "@/lib/auth/types";

export function UserMenu({ user }: { user: AuthUser }) {
  const t = useTranslations("common");
  const locale = useLocale();
  const router = useRouter();

  const logout = useMutation({
    mutationFn: () =>
      bffFetch("/api/bff/auth/logout", logoutDataSchema, { method: "POST", locale }),
    onSettled: () => {
      router.replace("/login");
      router.refresh();
    },
  });

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`${t("userMenu")}: ${user.displayName}`}
          className="inline-flex h-9 items-center gap-2 rounded-md border border-border px-2.5 text-sm text-foreground hover:bg-surface"
        >
          <User size={16} aria-hidden />
          <span className="max-w-[10rem] truncate">{user.displayName}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{user.email}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => logout.mutate()}
          disabled={logout.isPending}
          className="text-status-rose-fg"
        >
          <LogOut size={16} aria-hidden />
          {t("logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
