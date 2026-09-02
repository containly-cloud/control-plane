import 'i18next';

import type { defaultNS, resources } from '.';

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: typeof defaultNS;
    resources: (typeof resources)['pt-BR'];
    strictKeyChecks: true;
  }
}
