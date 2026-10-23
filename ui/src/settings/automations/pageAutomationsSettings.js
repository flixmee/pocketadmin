import { automationApprovalsList } from "./automationApprovalsList";
import { automationsList } from "./automationsList";

const tabStorageKey = "pbAutomationsTab";

const tabs = [
    {
        key: "automations",
        label: "Automations",
        icon: "ri-flashlight-line",
        description: "Manage workflows, toggle them on or off, and trigger manual runs.",
    },
    {
        key: "approvals",
        label: "Approvals",
        icon: "ri-shield-check-line",
        description:
            "Review the automation steps that wait for a manual approval. Select several to approve or reject them at once.",
    },
];

function loadSavedTab() {
    try {
        const saved = window.localStorage.getItem(tabStorageKey);
        return tabs.some((tab) => tab.key === saved) ? saved : "automations";
    } catch {
        return "automations";
    }
}

function saveTab(key) {
    try {
        window.localStorage.setItem(tabStorageKey, key);
    } catch {
        // storage unavailable (e.g. private mode) - the tab just isn't remembered
    }
}

export function pageAutomationsSettings() {
    app.store.title = "Automations";

    const data = store({
        activeTab: loadSavedTab(),
        resetList: null,
        automationsCount: null,
        pendingApprovalsCount: null,
    });

    function setTab(key) {
        data.activeTab = key;
        saveTab(key);
    }

    function refresh() {
        data.resetList = Date.now();
    }

    function tabCount(key) {
        return key === "approvals" ? data.pendingApprovalsCount : data.automationsCount;
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
                app.components.refreshButton({
                    className: "btn transparent secondary circle",
                    onclick: refresh,
                }),
                t.a(
                    { href: "#/automations/new", className: "btn" },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Create automation"),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.div(
                    { className: "tabs-header al-page-tabs", role: "tablist" },
                    tabs.map((tab) =>
                        t.button(
                            {
                                type: "button",
                                role: "tab",
                                className: () => `tab-item ${data.activeTab === tab.key ? "active" : ""}`,
                                ariaSelected: () => data.activeTab === tab.key,
                                onclick: () => setTab(tab.key),
                            },
                            t.i({ className: tab.icon, ariaHidden: true }),
                            t.span({ className: "txt" }, tab.label),
                            () => {
                                const count = tabCount(tab.key);
                                if (count === null) {
                                    return null;
                                }
                                const highlight = tab.key === "approvals" && count > 0;
                                return t.span(
                                    { className: `al-tab-count ${highlight ? "highlight" : ""}` },
                                    count,
                                );
                            },
                        )
                    ),
                ),
                t.p(
                    { className: "txt-hint al-tab-desc" },
                    () => tabs.find((tab) => tab.key === data.activeTab)?.description,
                ),
                // both lists stay mounted (hidden when inactive) so that the
                // counts and the realtime updates are kept up to date
                t.div(
                    { hidden: () => data.activeTab !== "automations" },
                    automationsList({
                        reset: () => data.resetList,
                        oncount: (count) => (data.automationsCount = count),
                    }),
                ),
                t.div(
                    { hidden: () => data.activeTab !== "approvals" },
                    automationApprovalsList({
                        reset: () => data.resetList,
                        oncount: (count) => (data.pendingApprovalsCount = count),
                    }),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
