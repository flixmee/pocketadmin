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
                // Automation definitions section
                t.div(
                    { className: "al-section-header" },
                    t.div(
                        { className: "al-section-header-left" },
                        t.div(
                            { className: "al-section-title-row" },
                            t.div({ className: "txt-lg" }, "Automation definitions"),
                            t.i({
                                className: "ri-refresh-line al-section-refresh-icon",
                                ariaHidden: true,
                                title: "Refresh",
                                onclick: resetAutomationsList,
                            }),
                        ),
                        t.div(
                            { className: "al-section-desc" },
                            "Manage workflows, toggle them on or off, and trigger manual runs.",
                        ),
                    ),
                    t.a(
                        {
                            href: "#/automations/new",
                            className: "al-section-action",
                        },
                        "Manage",
                        t.i({ className: "ri-arrow-right-s-line", ariaHidden: true }),
                    ),
                ),
                automationsList({
                    reset: () => data.resetList,
                }),
                // Pending approvals section
                t.div(
                    { className: "al-section-header m-t-base" },
                    t.div(
                        { className: "al-section-header-left" },
                        t.div(
                            { className: "al-section-title-row" },
                            t.div({ className: "txt-lg" }, "Pending approvals"),
                            t.i({
                                className: "ri-refresh-line al-section-refresh-icon",
                                ariaHidden: true,
                                title: "Refresh",
                                onclick: resetAutomationsList,
                            }),
                        ),
                        t.div(
                            { className: "al-section-desc" },
                            "Review and resolve automation steps that require manual approval.",
                        ),
                    ),
                ),
                automationApprovalsList({
                    reset: () => data.resetList,
                }),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
