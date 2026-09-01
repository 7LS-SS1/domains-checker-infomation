"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Copy, Loader2, ShieldAlert } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DateValue } from "@/components/ui/date-value";
import { ApiError } from "@/lib/api/envelope";
import { useCreateRegistrationToken } from "@/lib/probes/mutations";
import type { ProbeRegistrationToken } from "@/lib/probes/types";

export function CreateRegistrationTokenDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("probesPage.form");
  const tReveal = useTranslations("probesPage.reveal");
  const tCommon = useTranslations("common");
  const createToken = useCreateRegistrationToken();
  const [createdToken, setCreatedToken] = useState<ProbeRegistrationToken | undefined>();
  const [copied, setCopied] = useState(false);

  const schema = z.object({
    name: z.string().min(1),
    region_code: z.string().min(1),
    country_code: z.string().min(1),
    network_name: z.string().optional(),
    ttl_hours: z.number().int().min(1).max(720),
  });
  type FormValues = z.infer<typeof schema>;

  const { register, handleSubmit, reset } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", region_code: "", country_code: "", network_name: "", ttl_hours: 24 },
  });

  function handleClose(next: boolean) {
    onOpenChange(next);
    if (!next) {
      setCreatedToken(undefined);
      setCopied(false);
      reset();
    }
  }

  const onSubmit = handleSubmit((values) => {
    createToken.mutate(
      {
        name: values.name,
        region_code: values.region_code,
        country_code: values.country_code,
        network_name: values.network_name || undefined,
        ttl_seconds: values.ttl_hours * 3600,
      },
      { onSuccess: (result) => setCreatedToken(result) },
    );
  });

  async function handleCopy() {
    if (!createdToken) return;
    try {
      await navigator.clipboard.writeText(createdToken.token);
      setCopied(true);
    } catch {
      // Clipboard API can be unavailable — the token remains visible and manually selectable.
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent closeLabel={tCommon("cancel")}>
        {createdToken ? (
          <>
            <DialogTitle>{tReveal("title")}</DialogTitle>
            <DialogDescription>{tReveal("warning")}</DialogDescription>
            <div className="mt-4 space-y-3">
              <p
                role="alert"
                className="flex items-start gap-2 rounded-md border border-status-amber-border bg-status-amber-bg p-2 text-xs text-status-amber-fg"
              >
                <ShieldAlert size={14} className="mt-0.5 shrink-0" aria-hidden />
                {tReveal("warning")}
              </p>
              <div className="space-y-1.5">
                <Label htmlFor="revealed_token">{tReveal("tokenLabel")}</Label>
                <div className="flex gap-2">
                  <textarea
                    id="revealed_token"
                    readOnly
                    value={createdToken.token}
                    rows={3}
                    className="w-full rounded-md border border-border bg-surface p-2 font-mono text-xs text-foreground"
                    onFocus={(event) => event.currentTarget.select()}
                  />
                </div>
                <Button type="button" variant="secondary" size="sm" onClick={handleCopy}>
                  <Copy size={14} aria-hidden />
                  {copied ? tReveal("copied") : tReveal("copy")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {tReveal("expiresAt")}: <DateValue value={createdToken.expires_at} />
              </p>
              <div className="flex justify-end">
                <Button type="button" onClick={() => handleClose(false)}>
                  {tReveal("done")}
                </Button>
              </div>
            </div>
          </>
        ) : (
          <>
            <DialogTitle>{t("title")}</DialogTitle>
            <DialogDescription>{t("description")}</DialogDescription>
            <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
              {createToken.isError && (
                <p role="alert" className="text-sm text-status-rose-fg">
                  {createToken.error instanceof ApiError
                    ? createToken.error.message
                    : tCommon("loadError")}
                </p>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="probe_token_name">{t("name")}</Label>
                <Input id="probe_token_name" {...register("name")} />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label htmlFor="probe_token_region">{t("regionCode")}</Label>
                  <Input id="probe_token_region" {...register("region_code")} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="probe_token_country">{t("countryCode")}</Label>
                  <Input id="probe_token_country" {...register("country_code")} />
                </div>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="probe_token_network">{t("networkName")}</Label>
                <Input id="probe_token_network" {...register("network_name")} />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="probe_token_ttl">{t("ttlHours")}</Label>
                <Input
                  id="probe_token_ttl"
                  type="number"
                  min={1}
                  max={720}
                  {...register("ttl_hours", { valueAsNumber: true })}
                />
              </div>

              <div className="flex justify-end gap-2">
                <Button type="submit" disabled={createToken.isPending}>
                  {createToken.isPending && (
                    <Loader2 className="animate-spin" size={16} aria-hidden />
                  )}
                  {t("submit")}
                </Button>
              </div>
            </form>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
