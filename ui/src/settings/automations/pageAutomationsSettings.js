import { settingsSidebar } from "../settingsSidebar";
import { automationsList } from "./automationsList";

export function pageAutomationsSettings() {
    app.store.title = "Automations";

    const data = store({
        resetList: null,
    });

    function resetAutomationsList() {
        data.resetList = Date.now();
    }

    return t.div(
        { pbEvent: "pageAutomationsSettings", className: "page page-automations-settings" },
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
                    t.div({ className: "txt-lg" }, "Automation definitions"),
                    app.components.refreshButton({
                        className: "btn sm transparent secondary circle",
                        onclick: resetAutomationsList,
                    }),
                ),
                t.p(
                    { className: "txt-sm txt-hint m-b-sm" },
                    "Manage superuser automations, toggle them on or off, trigger manual runs, and expose webhook endpoints. ",
                    "The editor now uses structured step forms for supported triggers and actions.",
                ),
                automationsList({
                    reset: () => data.resetList,
                }),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
