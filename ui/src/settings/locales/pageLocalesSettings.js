import { settingsSidebar } from "../settingsSidebar";
import { localesList } from "./localesList";

export function pageLocalesSettings() {
    app.store.title = "Locales";

    const data = store({
        resetList: null,
    });

    function resetLocalesList() {
        data.resetList = Date.now();
    }

    return t.div(
        { pbEvent: "pageLocalesSettings", className: "page page-locales-settings" },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.div(
                    { className: "flex gap-10 m-b-sm" },
                    t.div({ className: "txt-lg" }, "Content locales"),
                    app.components.refreshButton({
                        className: "btn sm transparent secondary circle",
                        onclick: resetLocalesList,
                    }),
                ),
                t.p(
                    { className: "txt-sm txt-hint m-b-sm" },
                    "Configure the enabled locales used by localized collections and translation records.",
                ),
                localesList({
                    reset: () => data.resetList,
                }),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
