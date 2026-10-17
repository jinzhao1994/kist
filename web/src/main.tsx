import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider, initializeLanguage } from "./i18n";
import { App } from "./App";
import "@mantine/core/styles.css";
import "@mantine/dates/styles.css";
import "./style.css";
import { ThemeProvider, initializeAppearance } from "./theme";
initializeLanguage();
initializeAppearance();
const client = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: true } },
});
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <I18nProvider>
          <App />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
