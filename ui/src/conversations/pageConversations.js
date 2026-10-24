import { zaloConversations } from "./zaloConversations";

// the supported conversation channels (the availability comes from app.store.conversationChannels)
const channelTabs = [
    {
        id: "zalo",
        label: "Zalo",
        icon: "ri-chat-smile-2-line",
        render: (channel) => zaloConversations({ channel }),
    },
];

export function pageConversations(route) {
    app.store.title = "Conversations";

    const requestedId = route?.params?.channel || "";

    // the requested channel, otherwise the first active one
    const currentTab = () =>
        channelTabs.find((tab) => tab.id === requestedId)
        || channelTabs.find((tab) => channelState(tab.id)?.active)
        || channelTabs[0];

    // derived from the global channels state so that reloading it doesn't
    // re-create the channel view (and lose the opened conversation) unless
    // the channel availability actually changed
    const view = store({
        tabId: currentTab().id,
        active: !!channelState(currentTab().id)?.active,
    });
    let activeWatcher = null;

    return t.div(
        {
            pbEvent: "pageConversations",
            className: "page page-conversations",
            onmount: () => {
                activeWatcher = watch(
                    () => `${currentTab().id}:${!!channelState(currentTab().id)?.active}`,
                    () => {
                        const tabId = currentTab().id;
                        const active = !!channelState(tabId)?.active;
                        if (view.tabId !== tabId) {
                            view.tabId = tabId;
                        }
                        if (view.active !== active) {
                            view.active = active;
                        }
                    },
                );
            },
            onunmount: () => activeWatcher?.unwatch(),
        },
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
            ),
            t.div(
                { className: "tabs-header conversations-tabs", role: "tablist" },
                channelTabs.map((tab) =>
                    t.a(
                        {
                            href: `#/conversations/${tab.id}`,
                            role: "tab",
                            className: () => `tab-item ${view.tabId === tab.id ? "active" : ""}`,
                            ariaSelected: () => view.tabId === tab.id,
                        },
                        t.i({ className: tab.icon, ariaHidden: true }),
                        t.span({ className: "txt" }, tab.label),
                    )
                ),
            ),
            () => {
                const tab = channelTabs.find((item) => item.id === view.tabId) || channelTabs[0];
                if (!view.active) {
                    return t.div(null, () => channelUnavailable(tab, channelState(tab.id)));
                }

                return tab.render(() => channelState(tab.id));
            },
        ),
    );
}

function channelState(id) {
    return app.store.conversationChannels.find((channel) => channel.id === id);
}

function channelUnavailable(tab, state) {
    let message = `${tab.label} is not connected.`;
    if (!state) {
        message = `Loading ${tab.label}...`;
    } else if (!state.enabled) {
        message = `The ${tab.label} integration is not enabled.`;
    } else if (state.status && state.status !== "logged_in") {
        message = `The ${tab.label} account is not logged in (${state.status.replace(/_/g, " ")}).`;
    }

    return t.div(
        { className: "conversations-unavailable" },
        t.i({ className: `${tab.icon} conversations-unavailable-icon`, ariaHidden: true }),
        t.div({ className: "txt-bold" }, message),
        state
            ? t.a(
                { href: `#/settings/${tab.id}`, className: "btn sm secondary" },
                t.span({ className: "txt" }, `Open ${tab.label} settings`),
            )
            : null,
    );
}
