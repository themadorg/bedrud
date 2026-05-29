import { useIntl, FormattedMessage as IntlFormattedMessage } from 'react-intl';
import enMessages from '../locales/en.json';

export const messages = {
  en: enMessages,
};

export type Locale = keyof typeof messages;

export const defaultLocale: Locale = 'en';

export function useI18n() {
  const intl = useIntl();
  
  return {
    formatMessage: intl.formatMessage.bind(intl),
    formatDate: intl.formatDate.bind(intl),
    formatTime: intl.formatTime.bind(intl),
    formatNumber: intl.formatNumber.bind(intl),
    formatPlural: intl.formatPlural.bind(intl),
  };
}

// Convenience wrapper for FormattedMessage with better typing
export const FormattedMessage = IntlFormattedMessage;
