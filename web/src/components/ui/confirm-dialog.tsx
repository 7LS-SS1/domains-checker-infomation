"use client";

import { useEffect, useId, useState } from "react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  reasonLabel: string;
  reasonPlaceholder?: string;
  confirmLabel: string;
  cancelLabel: string;
  variant?: "default" | "destructive";
  isSubmitting?: boolean;
  errorMessage?: string;
  onConfirm: (reason: string) => void;
}

/**
 * Shared reason-required confirmation dialog for every destructive/effective
 * action (archive, restore, override create/revoke, import reject, probe
 * revoke, ...) per the Master Prompt UX rule. The confirm button stays
 * disabled until a non-empty reason is entered — the reason requirement is
 * enforced client-side here regardless of what the backend itself validates
 * (see web/API_GAPS.md GAP-04 on inconsistent backend reason enforcement).
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  reasonLabel,
  reasonPlaceholder,
  confirmLabel,
  cancelLabel,
  variant = "default",
  isSubmitting = false,
  errorMessage,
  onConfirm,
}: ConfirmDialogProps) {
  const [reason, setReason] = useState("");
  const reasonId = useId();

  // React's "adjust state during render" alternative to this effect was
  // tried here and measurably broke controlled-textarea typing under this
  // stack (React Compiler + Radix Dialog + userEvent) — verified by a
  // reproducible scrambled-input test failure. This effect is a legitimate
  // "synchronize with an external system" case (Radix's own open/close
  // lifecycle), so the stricter lint rule is suppressed deliberately here
  // rather than reintroducing that regression.
  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setReason("");
    }
  }, [open]);
  const trimmedReason = reason.trim();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={cancelLabel}>
        <DialogTitle>{title}</DialogTitle>
        {description && <DialogDescription>{description}</DialogDescription>}

        <form
          className="mt-4 space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (trimmedReason.length > 0 && !isSubmitting) {
              onConfirm(trimmedReason);
            }
          }}
        >
          <div className="space-y-1.5">
            <Label htmlFor={reasonId}>{reasonLabel}</Label>
            <textarea
              id={reasonId}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder={reasonPlaceholder}
              required
              rows={3}
              className="w-full rounded-md border border-border bg-background p-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>

          {errorMessage && (
            <p role="alert" className="text-sm text-status-rose-fg">
              {errorMessage}
            </p>
          )}

          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button type="button" variant="secondary">
                {cancelLabel}
              </Button>
            </DialogClose>
            <Button
              type="submit"
              variant={variant === "destructive" ? "destructive" : "primary"}
              disabled={trimmedReason.length === 0 || isSubmitting}
            >
              {confirmLabel}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
