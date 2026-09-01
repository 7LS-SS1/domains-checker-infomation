"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { useTranslations } from "next-intl";
import { SidebarNav } from "@/components/shell/sidebar-nav";

interface MobileDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Radix Dialog provides the focus trap and Esc-to-close behavior required
 * by web/FRONTEND_ARCHITECTURE.md section 9 out of the box.
 */
export function MobileDrawer({ open, onOpenChange }: MobileDrawerProps) {
  const t = useTranslations("common");

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content className="fixed inset-y-0 left-0 z-50 flex w-72 max-w-[85vw] flex-col bg-background shadow-lg focus:outline-none">
          <div className="flex items-center justify-between border-b border-border p-4">
            <Dialog.Title className="text-sm font-semibold text-foreground">
              {t("appName")}
            </Dialog.Title>
            <Dialog.Close
              aria-label={t("cancel")}
              className="rounded-md p-1.5 text-muted-foreground hover:bg-surface hover:text-foreground"
            >
              <X size={18} aria-hidden />
            </Dialog.Close>
          </div>
          <SidebarNav onNavigate={() => onOpenChange(false)} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
