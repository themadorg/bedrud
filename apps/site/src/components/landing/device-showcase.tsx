import { MacbookScroll } from "~/components/ui/macbook-scroll";
import { MobilePhoneScroll } from "~/components/ui/mobile-phone-scroll";
import { AndroidMockup, IPhoneMockup } from "~/components/ui/phone-mockup";
import type { Locale } from "../../i18n/utils";

export function DeviceShowcase({ lang: _lang }: { lang: Locale }) {
  return (
    <div className="relative pb-[6rem] md:pb-[32rem]">
      {/* Desktop: MacBook + flanking phones (md+) */}
      <div className="relative hidden md:block">
        <div className="pointer-events-none absolute inset-0 z-10 hidden items-end justify-center gap-0 lg:flex">
          <div className="w-32 shrink-0 xl:w-40">
            <IPhoneMockup>
              <img
                src="/preview/meeting-phone.png"
                alt=""
                className="size-full object-cover object-center"
              />
            </IPhoneMockup>
          </div>

          <div className="w-lg shrink-0" />

          <div className="w-32 shrink-0 xl:w-40">
            <AndroidMockup>
              <img
                src="/preview/meeting-phone-light.png"
                alt=""
                className="size-full object-cover object-center"
              />
            </AndroidMockup>
          </div>
        </div>

        <MacbookScroll showGradient={false}>
          <div className="size-full overflow-hidden bg-black">
            <img
              src="/preview/meeting-desktop.png"
              alt="Bedrud meeting"
              className="size-full object-cover object-center"
            />
          </div>
        </MacbookScroll>
      </div>

      {/* Mobile: Phone with scroll animation (<md) */}
      <MobilePhoneScroll>
        <img
          src="/preview/meeting-phone.png"
          alt="Bedrud meeting on phone"
          className="size-full object-cover object-center"
        />
      </MobilePhoneScroll>
    </div>
  );
}
