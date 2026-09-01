"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/envelope";
import { useCreateDomain } from "@/lib/domains/mutations";
import { BUSINESS_PRIORITIES, EXPECTED_CONTENT_MODES } from "@/lib/domains/types";

interface AddDomainDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AddDomainDialog({ open, onOpenChange }: AddDomainDialogProps) {
  const t = useTranslations("domains.form");
  const tCommon = useTranslations("common");
  const tStatus = useTranslations("domainStatus");
  const router = useRouter();
  const createDomain = useCreateDomain();
  const [serverError, setServerError] = useState<{ message: string; requestId?: string } | null>(
    null,
  );

  const schema = z.object({
    domain: z.string().min(1, t("domainRequired")),
    business_priority: z.enum(BUSINESS_PRIORITIES),
    expected_content_mode: z.enum(EXPECTED_CONTENT_MODES),
    monitoring_enabled: z.boolean(),
    expiration_at: z.string().optional(),
    notes: z.string(),
  });
  type FormValues = z.infer<typeof schema>;

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      domain: "",
      business_priority: "medium",
      expected_content_mode: "HTML",
      monitoring_enabled: true,
      expiration_at: "",
      notes: "",
    },
  });

  const onSubmit = handleSubmit((values) => {
    setServerError(null);
    createDomain.mutate(
      {
        domain: values.domain,
        business_priority: values.business_priority,
        expected_content_mode: values.expected_content_mode,
        monitoring_enabled: values.monitoring_enabled,
        expiration_at: values.expiration_at || null,
        notes: values.notes,
      },
      {
        onSuccess: (created) => {
          reset();
          onOpenChange(false);
          router.push(`/domains/${created.id}`);
        },
        onError: (error) => {
          if (error instanceof ApiError) {
            setServerError({ message: error.message, requestId: error.requestId });
          } else {
            setServerError({ message: tCommon("loadError") });
          }
        },
      },
    );
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={tCommon("cancel")}>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description")}</DialogDescription>

        <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
          {serverError && (
            <div
              role="alert"
              className="rounded-md border border-status-rose-border bg-status-rose-bg px-3 py-2 text-sm text-status-rose-fg"
            >
              <p>{serverError.message}</p>
              {serverError.requestId && (
                <p className="mt-1 text-xs opacity-80">
                  {tCommon("requestId")}: {serverError.requestId}
                </p>
              )}
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="domain">{t("domainLabel")}</Label>
            <Input
              id="domain"
              placeholder={t("domainPlaceholder")}
              aria-invalid={!!errors.domain}
              {...register("domain")}
            />
            {errors.domain && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {errors.domain.message}
              </p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="business_priority">{t("businessPriorityLabel")}</Label>
              <select
                id="business_priority"
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
              <Label htmlFor="expected_content_mode">{t("expectedContentModeLabel")}</Label>
              <select
                id="expected_content_mode"
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

          <div className="space-y-1.5">
            <Label htmlFor="expiration_at">{t("expirationLabel")}</Label>
            <Input id="expiration_at" type="date" {...register("expiration_at")} />
          </div>

          <div className="flex items-center gap-2">
            <input
              id="monitoring_enabled"
              type="checkbox"
              className="h-4 w-4 rounded border-border"
              {...register("monitoring_enabled")}
            />
            <Label htmlFor="monitoring_enabled" className="font-normal">
              {t("monitoringEnabledLabel")}
            </Label>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="notes">{t("notesLabel")}</Label>
            <textarea
              id="notes"
              rows={2}
              className="w-full rounded-md border border-border bg-background p-2 text-sm text-foreground"
              {...register("notes")}
            />
          </div>

          <div className="flex justify-end gap-2">
            <Button type="submit" disabled={createDomain.isPending}>
              {createDomain.isPending ? (
                <>
                  <Loader2 className="animate-spin" size={16} aria-hidden />
                  {tCommon("creating")}
                </>
              ) : (
                t("submit")
              )}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
