import { automationApprovalsList } from "./automationApprovalsList";
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
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
                t.div({ className: "flex-fill" }),
                t.a(
                    { href: "#/automations/new", className: "btn" },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Create automation"),
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
                t.div(
                    { className: "flex gap-10 m-t-base m-b-sm" },
                    t.div({ className: "txt-lg" }, "Pending approvals"),
                    app.components.refreshButton({
                        className: "btn sm transparent secondary circle",
                        onclick: resetAutomationsList,
                    }),
                ),
                automationApprovalsList({
                    reset: () => data.resetList,
                }),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
