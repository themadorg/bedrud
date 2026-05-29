import { Menu } from "lucide-react";
import * as React from "react";
import { Button } from "~/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetOverlay,
  SheetTitle,
  SheetTrigger,
} from "~/components/ui/sheet";
import { GITHUB_URL } from "~/lib/config";
import type { Locale } from "../../i18n/utils";
import { getDir, t } from "../../i18n/utils";
import { GitHubIcon } from "../landing/github-icon";
import { Search } from "./search";
import { Sidebar } from "./sidebar";

interface DocsMobileSidebarProps {
  lang: Locale;
  currentSlug?: string;
}

export function DocsMobileSidebar({
  lang,
  currentSlug = "",
}: DocsMobileSidebarProps) {
  const [open, setOpen] = React.useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" className="lg:hidden">
          <Menu className="size-5" aria-hidden="true" />
          <span className="sr-only">{t(lang, "docs.toggleMenu")}</span>
        </Button>
      </SheetTrigger>
      <SheetOverlay />
      <SheetContent
        side={getDir(lang) === "rtl" ? "right" : "left"}
        className="w-3/4 max-w-sm p-0"
        closeLabel={t(lang, "a11y.closeMenu")}
      >
        <SheetTitle className="sr-only">
          {t(lang, "docs.toggleMenu")}
        </SheetTitle>
        <SheetDescription className="sr-only">
          {t(lang, "docs.toggleMenu")}
        </SheetDescription>
        <div className="flex flex-col h-full">
          <div className="flex-1 overflow-y-auto">
            <div className="flex items-center gap-2 px-3 pt-3 pb-2">
              <Search lang={lang} />
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noreferrer"
                aria-label="GitHub Repository"
                className="inline-flex size-8 shrink-0 items-center justify-center rounded-md border bg-background text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
              >
                <GitHubIcon className="size-4" />
              </a>
            </div>
            <Sidebar lang={lang} currentSlug={currentSlug} />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
