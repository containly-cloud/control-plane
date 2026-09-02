import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

import enUS from './locales/en-US.json';
import ptBR from './locales/pt-BR.json';

export const supportedLocales = ['pt-BR', 'en-US'] as const;
export type Locale = (typeof supportedLocales)[number];

export const defaultLocale: Locale = 'pt-BR';
export const defaultNS = 'translation';
type PortugueseResources = typeof ptBR;
type TranslationResources<T> = {
  [Key in keyof T]: T[Key] extends string
    ? string
    : TranslationResources<T[Key]>;
};

const ptBRMessages: TranslationResources<PortugueseResources> = ptBR;
const enUSMessages: TranslationResources<PortugueseResources> = enUS;

export const resources = {
  'pt-BR': { translation: ptBRMessages },
  'en-US': { translation: enUSMessages },
} as const;

const localeStorageKey = 'containly.locale';

function resolveLocale(candidates: readonly string[]): Locale {
  for (const candidate of candidates) {
    const normalized = candidate.replace('_', '-').toLowerCase();
    if (normalized === 'en-us' || normalized.startsWith('en-')) {
      return 'en-US';
    }
    if (normalized === 'pt-br' || normalized.startsWith('pt-')) {
      return 'pt-BR';
    }
  }
  return defaultLocale;
}

function getInitialLocale(): Locale {
  try {
    const savedLocale = window.localStorage.getItem(localeStorageKey);
    if (savedLocale && supportedLocales.includes(savedLocale as Locale)) {
      return savedLocale as Locale;
    }
  } catch {
    // The browser preference is still available when storage is disabled.
  }
  return resolveLocale(navigator.languages);
}

export function toSupportedLocale(language: string | undefined): Locale {
  return resolveLocale(language ? [language] : []);
}

export function selectLocale(locale: Locale) {
  try {
    window.localStorage.setItem(localeStorageKey, locale);
  } catch {
    // The selected locale remains active for the current page session.
  }
  return i18n.changeLanguage(locale);
}

void i18n.use(initReactI18next).init({
  resources,
  lng: getInitialLocale(),
  fallbackLng: defaultLocale,
  supportedLngs: supportedLocales,
  defaultNS,
  fallbackNS: defaultNS,
  interpolation: { escapeValue: false },
  returnNull: false,
  saveMissing: false,
});

export default i18n;
