import { ArrowLeft } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { sections } from "@/content/docs/sidebar";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "~/components/ui/accordion";
import { cn } from "~/lib/utils";
import { type Locale, t } from "../../i18n/utils";

interface SidebarProps {
  lang: Locale;
  currentSlug?: string;
}

function Sidebar({ lang, currentSlug = "" }: SidebarProps) {
  const [currentHash, setCurrentHash] = useState("");
  const sidebarRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const handleHashChange = () => {
      setCurrentHash(window.location.hash);
    };

    const handleSidebarClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      const anchor = target.closest("a");
      if (!anchor) return;

      try {
        const url = new URL(anchor.href, window.location.origin);
        if (url.pathname === window.location.pathname && url.hash) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
          window.location.hash = url.hash;
        }
      } catch (_err) {
        // Ignore invalid URLs
      }
    };

    handleHashChange();
    window.addEventListener("hashchange", handleHashChange);
    document.addEventListener("astro:page-load", handleHashChange);

    const sidebarEl = sidebarRef.current;
    if (sidebarEl) {
      sidebarEl.addEventListener("click", handleSidebarClick, {
        capture: true,
      });
    }

    return () => {
      window.removeEventListener("hashchange", handleHashChange);
      document.removeEventListener("astro:page-load", handleHashChange);
      if (sidebarEl) {
        sidebarEl.removeEventListener("click", handleSidebarClick, {
          capture: true,
        });
      }
    };
  }, []);

  const activeSection = sections.find((section) =>
    section.items.some((item) => item.slug === currentSlug),
  );

  const openSections = activeSection ? [activeSection.title] : [];

  const getHref = (slug: string) => {
    const isReferencePage =
      currentSlug === "api/api-refrence" || currentSlug === "api-docs";
    if (slug === "api/auth")
      return isReferencePage
        ? "#tag/auth"
        : `/${lang}/docs/api/api-refrence#tag/auth`;
    if (slug === "api/rooms")
      return isReferencePage
        ? "#tag/rooms"
        : `/${lang}/docs/api/api-refrence#tag/rooms`;
    if (slug === "api/admin")
      return isReferencePage
        ? "#tag/admin"
        : `/${lang}/docs/api/api-refrence#tag/admin`;
    if (slug === "api/system")
      return isReferencePage
        ? "#tag/system"
        : `/${lang}/docs/api/api-refrence#tag/system`;
    if (slug === "api/health")
      return isReferencePage
        ? "#tag/health"
        : `/${lang}/docs/api/api-refrence#tag/health`;
    if (slug === "api/models")
      return isReferencePage
        ? "#models"
        : `/${lang}/docs/api/api-refrence#models`;
    if (slug === "api/api-refrence" || slug === "api/reference")
      return isReferencePage ? "#" : `/${lang}/docs/api/api-refrence`;
    return `/${lang}/docs/${slug}`;
  };

  const getIsActive = (slug: string) => {
    if (currentSlug === "api/api-refrence" || currentSlug === "api-docs") {
      if (slug === "api/api-refrence" || slug === "api/reference") {
        return (
          !currentHash ||
          (!currentHash.startsWith("#tag/auth") &&
            !currentHash.startsWith("#tag/rooms") &&
            !currentHash.startsWith("#tag/admin") &&
            !currentHash.startsWith("#tag/system") &&
            !currentHash.startsWith("#tag/health") &&
            !currentHash.startsWith("#models"))
        );
      }
      if (slug === "api/auth") {
        return currentHash.startsWith("#tag/auth");
      }
      if (slug === "api/rooms") {
        return currentHash.startsWith("#tag/rooms");
      }
      if (slug === "api/admin") {
        return currentHash.startsWith("#tag/admin");
      }
      if (slug === "api/system") {
        return currentHash.startsWith("#tag/system");
      }
      if (slug === "api/health") {
        return currentHash.startsWith("#tag/health");
      }
      if (slug === "api/models") {
        return currentHash.startsWith("#models");
      }
    }
    return slug === currentSlug;
  };

  return (
    <aside ref={sidebarRef} className="border-e" suppressHydrationWarning>
      <div className="scroll-area h-[calc(100vh-4rem)] py-6 pe-4">
        <div className="space-y-3">
          <div className="pb-4">
            <a
              href={`/${lang}`}
              className="flex items-center gap-2 px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              <span>{t(lang, "docs.backToHome")}</span>
            </a>
          </div>
          <div className="pb-4">
            <h3 className="px-4 text-sm font-semibold">
              {t(lang, "docs.documentation")}
            </h3>
          </div>
          <Accordion
            type="multiple"
            defaultValue={openSections}
            className="space-y-2"
          >
            {sections.map((section) => (
              <AccordionItem key={section.title} value={section.title}>
                <AccordionTrigger className="ps-4 py-2 text-sm font-semibold hover:no-underline">
                  {t(lang, `docs.sections.${section.titleKey}`)}
                </AccordionTrigger>
                <AccordionContent className="pt-0 pb-0">
                  <nav className="space-y-1.5">
                    {section.items.map((item) => {
                      const isActive = getIsActive(item.slug);
                      return (
                        <a
                          key={item.slug}
                          href={getHref(item.slug)}
                          className={cn(
                            "block rounded-md ps-5 pe-3 py-2 text-sm transition-colors ms-2",
                            isActive
                              ? "bg-accent text-accent-foreground font-medium"
                              : "text-muted-foreground hover:bg-accent/50 hover:text-accent-foreground",
                          )}
                        >
                          {t(lang, `docs.sidebarItems.${item.slug}`)}
                        </a>
                      );
                    })}
                  </nav>
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        </div>
      </div>
    </aside>
  );
}

export { Sidebar };
