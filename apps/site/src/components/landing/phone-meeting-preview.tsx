import { Mic, MicOff, Phone, Video } from "lucide-react";
import type { Locale } from "../../i18n/utils";

/* ------------------------------------------------------------------ */
/*  Shared data (reuses same images as MeetingPreview)                 */
/* ------------------------------------------------------------------ */

const participants = {
  sarah: {
    name: "Shahram Farhadi",
    image: "/preview/avatars/athlete.png",
  },
  alex: {
    name: "Kiarash Rostami",
    image: "/preview/avatars/bored.png",
  },
  jordan: {
    name: "Parsa Kaviani",
    image: "/preview/avatars/cyborg.png",
  },
  marcus: {
    name: "Shirin Golestan",
    image: "/preview/avatars/architect.png",
  },
  self: {
    name: "You",
    image: "/preview/avatars/athlete.png",
  },
};

/* ------------------------------------------------------------------ */
/*  iPhone meeting – Speaker view                                      */
/* ------------------------------------------------------------------ */

export function IPhoneMeetingPreview({ lang: _lang }: { lang: Locale }) {
  return (
    <div
      aria-hidden="true"
      className="flex size-full select-none flex-col bg-[#0E1220]"
    >
      {/* Status bar spacer */}
      <div className="h-12 shrink-0" />

      {/* Main speaker */}
      <div className="relative flex-1 overflow-hidden">
        <img
          src={participants.sarah.image}
          alt=""
          className="size-full object-cover"
          loading="lazy"
        />
        {/* Name overlay */}
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent px-3 pb-2 pt-6">
          <span className="text-[10px] font-medium text-white/90">
            {participants.sarah.name}
          </span>
        </div>

        {/* Self PiP */}
        <div className="absolute end-2 top-2 w-[30%] overflow-hidden rounded-lg border border-white/10 shadow-lg">
          <div className="aspect-[3/4]">
            <img
              src={participants.self.image}
              alt=""
              className="size-full object-cover"
              loading="lazy"
            />
          </div>
        </div>
      </div>

      {/* Controls */}
      <div className="flex items-center justify-center gap-4 bg-[#0a0f1e] py-3">
        <div className="flex size-9 items-center justify-center rounded-full bg-white/10">
          <Mic className="size-4 text-white/80" />
        </div>
        <div className="flex size-9 items-center justify-center rounded-full bg-white/10">
          <Video className="size-4 text-white/80" />
        </div>
        <div className="flex size-9 items-center justify-center rounded-full bg-red-500">
          <Phone className="size-4 rotate-[135deg] text-white" />
        </div>
      </div>

      {/* Home indicator spacer */}
      <div className="h-5 shrink-0 bg-[#0a0f1e]" />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Android meeting – Gallery view                                     */
/* ------------------------------------------------------------------ */

export function AndroidMeetingPreview({ lang: _lang }: { lang: Locale }) {
  const grid = [
    participants.sarah,
    participants.alex,
    participants.jordan,
    participants.marcus,
  ];

  return (
    <div
      aria-hidden="true"
      className="flex size-full select-none flex-col bg-[#0E1220]"
    >
      {/* Status bar spacer */}
      <div className="h-8 shrink-0" />

      {/* 2x2 grid */}
      <div className="flex-1 grid grid-cols-2 gap-1 p-1">
        {grid.map((p) => (
          <div key={p.name} className="relative overflow-hidden rounded-md">
            <img
              src={p.image}
              alt=""
              className="size-full object-cover"
              loading="lazy"
            />
            <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent px-1.5 pb-1 pt-3">
              <span className="text-[8px] font-medium text-white/80">
                {p.name}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Controls */}
      <div className="flex items-center justify-center gap-3 bg-[#0a0f1e] py-2.5">
        <div className="flex size-8 items-center justify-center rounded-full bg-white/10">
          <MicOff className="size-3.5 text-red-400" />
        </div>
        <div className="flex size-8 items-center justify-center rounded-full bg-white/10">
          <Video className="size-3.5 text-white/80" />
        </div>
        <div className="flex size-8 items-center justify-center rounded-full bg-red-500">
          <Phone className="size-3.5 rotate-[135deg] text-white" />
        </div>
      </div>

      {/* Nav bar spacer */}
      <div className="h-4 shrink-0 bg-[#0a0f1e]" />
    </div>
  );
}
