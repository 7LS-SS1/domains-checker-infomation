"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/envelope";
import { usePatchDomain } from "@/lib/domains/mutations";
import {
  BUSINESS_PRIORITIES,
  EXPECTED_CONTENT_MODES,
  RENEWAL_DECISIONS,
  type Domain,
} from "@/lib/domains/types";

interface EditDomainDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  domain: Domain;
}

/**
 * PATCH sends the domain's `version` the page last loaded. A 409
 * VERSION_CONFLICT (someone else changed the record meanwhile) is shown
 * explicitly with a reload prompt — never silently retried or overwritten,
 * per the Master Prompt's optimistic-concurrency rule.
 */
export function EditDomainDialog({ open, onOpenChange, domain }: EditDomainDialogProps) {
  const t = useTranslations("domainDetail.edit");
  const tCommon = useTranslations("common");
  const tStatus = useTranslations("domainStatus");
  const tForm = useTranslations("domains.form");
  const tDomains = useTranslations("domains");
  const tConflict = useTranslations("domains.conflict");
  const patchDomain = usePatchDomain();

  const schema = z.object({
    business_priority: z.enum(BUSINESS_PRIORITIES),
    expected_content_mode: z.enum(EXPECTED_CONTENT_MODES),
    monitoring_enabled: z.boolean(),
    renewal_decision: z.enum(RENEWAL_DECISIONS),
    notes: z.string(),
    reason: z.string().min(1, tCommon("reasonPlaceholder")),
  });
  type FormValues = z.infer<typeof schema>;

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      business_priority: domain.business_priority as FormValues["business_priority"],
      expected_content_mode: domain.expected_content_mode as FormValues["expected_content_mode"],
      monitoring_enabled: domain.monitoring_enabled,
      renewal_decision: domain.renewal_decision as FormValues["renewal_decision"],
      notes: domain.notes,
      reason: "",
    },
  });

  const isConflict =
    patchDomain.error instanceof ApiError && patchDomain.error.code === "VERSION_CONFLICT";

  const onSubmit = handleSubmit((values) => {
    patchDomain.mutate(
      { domainId: domain.id, version: domain.version, ...values },
      { onSuccess: () => onOpenChange(false) },
    );
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={tCommon("cancel")}>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description")}</DialogDescription>

        <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
          {patchDomain.isError && (
            <div
              role="alert"
              className="rounded-md border border-status-rose-border bg-status-rose-bg px-3 py-2 text-sm text-status-rose-fg"
            >
              {isConflict ? (
                <>
                  <p className="font-medium">{tConflict("title")}</p>
                  <p>{tConflict("description")}</p>
                </>
              ) : (
                <p>
                  {patchDomain.error instanceof ApiError
                    ? patchDomain.error.message
                    : tCommon("loadError")}
                </p>
              )}
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="edit_business_priority">{tForm("businessPriorityLabel")}</Label>
              <select
                id="edit_business_priority"
                className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm text-foreground"
                {...register("business_priority")}
              >
                {BUSINESS_PRIORITIES.map((priority) => (
                  <option key={priority} value={priority}>
                    {tStatus(`priority.${priority}`)}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="edit_expected_content_mode">
                {tForm("expectedContentModeLabel")}
              </Label>
              <select
                id="edit_expected_content_mode"
                className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm text-foreground"
                {...register("expected_content_mode")}
              >
                {EXPECTED_CONTENT_MODES.map((mode) => (
                  <option key={mode} value={mode}>
                    {mode}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <input
              id="edit_monitoring_enabled"
              type="checkbox"
              className="h-4 w-4 rounded border-border"
              {...register("monitoring_enabled")}
            />
            <Label htmlFor="edit_monitoring_enabled" className="font-normal">
              {tForm("monitoringEnabledLabel")}
            </Label>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="edit_renewal_decision">{tDomains("columns.renewalDecision")}</Label>
            <select
              id="edit_renewal_decision"
              className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm text-foreground"
              {...register("renewal_decision")}
            >
              {RENEWAL_DECISIONS.map((decision) => (
                <option key={decision} value={decision}>
                  {tDomains(`renewalDecision.${decision}`)}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="edit_notes">{tForm("notesLabel")}</Label>
            <textarea
              id="edit_notes"
              rows={2}
              className="w-full rounded-md border border-border bg-background p-2 text-sm text-foreground"
              {...register("notes")}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="edit_reason">{tCommon("reason")}</Label>
            <Input
              id="edit_reason"
              placeholder={tCommon("reasonPlaceholder")}
              {...register("reason")}
            />
            {errors.reason && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {errors.reason.message}
              </p>
            )}
          </div>

          <div className="flex justify-end gap-2">
            <Button type="submit" disabled={patchDomain.isPending}>
              {patchDomain.isPending && <Loader2 className="animate-spin" size={16} aria-hidden />}
              {t("submit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
