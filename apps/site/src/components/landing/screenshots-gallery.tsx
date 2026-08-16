import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "~/components/ui/dialog";
import { cn } from "~/lib/utils";
import { type Locale, t } from "../../i18n/utils";

type Category = "meeting" | "chat" | "admin" | "auth";
type Theme = "light" | "dark";
type Device = "desktop" | "mobile";

interface Shot {
  src: string;
  title: string;
  category: Category;
  theme: Theme;
  device: Device;
}

const SHOTS: Shot[] = [
  { src: "/preview/gallery/meeting-grid__dark__desktop.png", title: "Meeting grid", category: "meeting", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-grid__light__desktop.png", title: "Meeting grid", category: "meeting", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-grid__dark__mobile.png", title: "Meeting grid", category: "meeting", theme: "dark", device: "mobile" },
  { src: "/preview/gallery/meeting-grid__light__mobile.png", title: "Meeting grid", category: "meeting", theme: "light", device: "mobile" },
  { src: "/preview/gallery/meeting-welcome__dark__desktop.png", title: "Welcome screen", category: "meeting", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-welcome__light__desktop.png", title: "Welcome screen", category: "meeting", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-participants__dark__desktop.png", title: "Participants", category: "meeting", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-participants__light__desktop.png", title: "Participants", category: "meeting", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-screenshare__dark__desktop.png", title: "Screen share", category: "meeting", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-screenshare__light__desktop.png", title: "Screen share", category: "meeting", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-info__dark__desktop.png", title: "Room info", category: "meeting", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-info__light__desktop.png", title: "Room info", category: "meeting", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-chat__dark__desktop.png", title: "Meeting chat", category: "chat", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/meeting-chat__light__desktop.png", title: "Meeting chat", category: "chat", theme: "light", device: "desktop" },
  { src: "/preview/gallery/meeting-chat__dark__mobile.png", title: "Meeting chat", category: "chat", theme: "dark", device: "mobile" },
  { src: "/preview/gallery/meeting-chat__light__mobile.png", title: "Meeting chat", category: "chat", theme: "light", device: "mobile" },
  { src: "/preview/gallery/landing__dark__desktop.png", title: "App landing", category: "auth", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/landing__light__desktop.png", title: "App landing", category: "auth", theme: "light", device: "desktop" },
  { src: "/preview/gallery/auth-login__dark__desktop.png", title: "Sign in", category: "auth", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/auth-login__light__desktop.png", title: "Sign in", category: "auth", theme: "light", device: "desktop" },
  { src: "/preview/gallery/dashboard__dark__desktop.png", title: "Dashboard", category: "admin", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/dashboard__light__desktop.png", title: "Dashboard", category: "admin", theme: "light", device: "desktop" },
  { src: "/preview/gallery/admin-overview__dark__desktop_1.png", title: "Admin overview", category: "admin", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/admin-overview__light__desktop_1.png", title: "Admin overview", category: "admin", theme: "light", device: "desktop" },
  { src: "/preview/gallery/admin-settings-auth__dark__desktop_1.png", title: "Admin settings", category: "admin", theme: "dark", device: "desktop" },
  { src: "/preview/gallery/admin-settings-auth__light__desktop_1.png", title: "Admin settings", category: "admin", theme: "light", device: "desktop" },
];

const CATEGORIES: { id: "all" | Category; key: string }[] = [
  { id: "all", key: "screenshotsPage.filterAll" },
  { id: "meeting", key: "screenshotsPage.filterMeeting" },
  { id: "chat", key: "screenshotsPage.filterChat" },
  { id: "admin", key: "screenshotsPage.filterAdmin" },
  { id: "auth", key: "screenshotsPage.filterAuth" },
];

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "border px-3 py-1.5 text-xs font-medium transition-colors",
        active
          ? "border-primary bg-primary text-primary-foreground"
          : "border-border bg-background text-muted-foreground hover:border-foreground/30 hover:text-foreground",
      )}
    >
      {children}
    </button>
  );
}

export function ScreenshotsGallery({ lang }: { lang: Locale }) {
  const [category, setCategory] = useState<"all" | Category>("all");
  const [theme, setTheme] = useState<"all" | Theme>("all");
  const [device, setDevice] = useState<"all" | Device>("all");
  const [open, setOpen] = useState<Shot | null>(null);

  const shots = useMemo(
    () =>
      SHOTS.filter((s) => {
        if (category !== "all" && s.category !== category) return false;
        if (theme !== "all" && s.theme !== theme) return false;
        if (device !== "all" && s.device !== device) return false;
        return true;
      }),
    [category, theme, device],
  );

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        {CATEGORIES.map((c) => (
          <FilterChip key={c.id} active={category === c.id} onClick={() => setCategory(c.id)}>
            {t(lang, c.key)}
          </FilterChip>
        ))}
        <span className="mx-1 hidden h-5 w-px bg-border sm:block" />
        <FilterChip active={device === "all"} onClick={() => setDevice("all")}>
          {t(lang, "screenshotsPage.filterAll")}
        </FilterChip>
        <FilterChip active={device === "desktop"} onClick={() => setDevice("desktop")}>
          {t(lang, "screenshotsPage.filterDesktop")}
        </FilterChip>
        <FilterChip active={device === "mobile"} onClick={() => setDevice("mobile")}>
          {t(lang, "screenshotsPage.filterMobile")}
        </FilterChip>
        <span className="mx-1 hidden h-5 w-px bg-border sm:block" />
        <FilterChip active={theme === "all"} onClick={() => setTheme("all")}>
          {t(lang, "screenshotsPage.filterAll")}
        </FilterChip>
        <FilterChip active={theme === "light"} onClick={() => setTheme("light")}>
          {t(lang, "screenshotsPage.filterLight")}
        </FilterChip>
        <FilterChip active={theme === "dark"} onClick={() => setTheme("dark")}>
          {t(lang, "screenshotsPage.filterDark")}
        </FilterChip>
      </div>

      <ul className="mt-8 grid gap-6 sm:grid-cols-2">
        {shots.map((shot) => (
          <li key={shot.src}>
            <button
              type="button"
              onClick={() => setOpen(shot)}
              className="group w-full border border-border bg-muted/20 text-start"
            >
              <div
                className={cn(
                  "overflow-hidden bg-black",
                  shot.device === "mobile" ? "flex justify-center py-6" : "",
                )}
              >
                <img
                  src={shot.src}
                  alt={shot.title}
                  className={cn(
                    "block",
                    shot.device === "mobile" ? "h-[28rem] w-auto" : "h-auto w-full",
                  )}
                  loading="lazy"
                />
              </div>
              <div className="flex items-center justify-between border-t border-border px-3 py-2">
                <span className="text-sm font-medium">{shot.title}</span>
                <span className="text-[11px] uppercase tracking-wider text-muted-foreground">
                  {shot.theme} · {shot.device}
                </span>
              </div>
            </button>
          </li>
        ))}
      </ul>

      <Dialog open={Boolean(open)} onOpenChange={(v) => !v && setOpen(null)}>
        <DialogContent className="max-h-[92vh] max-w-[min(96vw,72rem)] overflow-auto rounded-none border-border p-0">
          <DialogTitle className="sr-only">{open?.title}</DialogTitle>
          {open && (
            <img src={open.src} alt={open.title} className="mx-auto max-h-[90vh] w-auto max-w-full" />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
