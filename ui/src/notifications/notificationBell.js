export function notificationBell() {
    const dropdownId = "notifications-dropdown";

    return [
        t.button(
            {
                type: "button",
                className: "header-link notification-bell",
                title: () => notificationBellLabel(),
                ariaLabel: () => notificationBellLabel(),
                "html-popovertarget": dropdownId,
            },
            t.i({ className: "ri-notification-3-line", ariaHidden: true }),
            () => {
                const count = app.store.unreadNotifications;
                if (!count) {
                    return;
                }

                return t.span({ className: "notification-badge" }, formatNotificationCount(count));
            },
        ),
        t.div(
            {
                pbEvent: "notificationsDropdown",
                id: dropdownId,
                className: "dropdown notifications-dropdown",
                popover: "auto",
            },
            t.div(
                { className: "notifications-dropdown-header" },
                t.strong(null, "Notifications"),
                t.button(
                    {
                        type: "button",
                        className: "btn btn-sm transparent secondary",
                        disabled: () => !app.store.unreadNotifications || app.store.isMarkingNotificationsRead,
                        onclick: async (e) => {
                            e.preventDefault();
                            await app.store.markAllNotificationsRead();
                        },
                    },
                    t.i({ className: "ri-check-double-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Mark all read"),
                ),
            ),
            () => {
                if (app.store.isLoadingNotifications && !app.store.notifications.length) {
                    return t.div({ className: "notifications-empty" }, t.span({ className: "loader" }));
                }

                if (!app.store.notifications.length) {
                    return t.div({ className: "notifications-empty" }, "No notifications");
                }

                return app.store.notifications.map((notification) => {
                    return notificationRow(notification);
                });
            },
        ),
    ];
}

function notificationRow(notification) {
    const approvalId = notificationApprovalId(notification);
    const rowContent = [
        t.span({
            className: () => `notification-severity ${notification.severity || "info"}`,
            ariaHidden: true,
        }),
        t.div(
            { className: "notification-content" },
            t.div({ className: "notification-title txt-ellipsis" }, notification.title || "Untitled"),
            () => {
                if (notification.message) {
                    return t.div({ className: "notification-message" }, notification.message);
                }
            },
            t.time(
                { className: "notification-date", dateTime: notification.created },
                formatNotificationDate(notification.created),
            ),
            () => notificationApprovalActions(notification),
        ),
    ];

    return t.div(
        {
            className: () => `notification-row ${notification.read ? "" : "unread"}`,
        },
        notificationRowBody(notification, rowContent, approvalId),
        t.button(
            {
                type: "button",
                className: "notification-read-btn",
                title: "Mark as read",
                ariaLabel: "Mark as read",
                hidden: () => notification.read,
                disabled: () => app.store.isMarkingNotificationsRead,
                onclick: (e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    app.store.markNotificationRead(notification.id);
                },
            },
            t.i({ className: "ri-check-line", ariaHidden: true }),
        ),
    );
}

function notificationRowBody(notification, rowContent, approvalId) {
    if (notification.actionUrl && !approvalId) {
        return t.a(
            {
                className: "notification-row-body clickable",
                href: notification.actionUrl,
                target: () => notification.actionUrl.startsWith("#/") ? undefined : "_blank",
                rel: () => notification.actionUrl.startsWith("#/") ? undefined : "noopener noreferrer",
                onclick: (e) => {
                    e.target.closest(".dropdown")?.hidePopover();
                    if (!notification.read) {
                        app.store.markNotificationRead(notification.id);
                    }
                },
            },
            rowContent,
        );
    }

    if (approvalId) {
        return t.div({ className: "notification-row-body with-actions" }, rowContent);
    }

    return t.button(
        {
            type: "button",
            className: "notification-row-body clickable",
            onclick: () => {
                if (!notification.read) {
                    app.store.markNotificationRead(notification.id);
                }
            },
        },
        rowContent,
    );
}

function notificationApprovalActions(notification) {
    const approvalId = notificationApprovalId(notification);
    if (!approvalId || notification?.data?.approvalStatus && notification.data.approvalStatus != "pending") {
        return;
    }

    const resolvingDecision = app.store.resolvingNotificationApprovals[approvalId];

    return t.div(
        { className: "notification-actions" },
        t.button(
            {
                type: "button",
                className: () => `notification-action-btn success ${resolvingDecision == "approved" ? "loading" : ""}`,
                title: "Approve",
                ariaLabel: "Approve",
                disabled: () => !!resolvingDecision,
                onclick: (e) => confirmApprovalDecision(e, notification, "approved"),
            },
            t.i({ className: "ri-check-line", ariaHidden: true }),
            t.span({ className: "txt" }, "Approve"),
        ),
        t.button(
            {
                type: "button",
                className: () => `notification-action-btn danger ${resolvingDecision == "rejected" ? "loading" : ""}`,
                title: "Reject",
                ariaLabel: "Reject",
                disabled: () => !!resolvingDecision,
                onclick: (e) => confirmApprovalDecision(e, notification, "rejected"),
            },
            t.i({ className: "ri-close-line", ariaHidden: true }),
            t.span({ className: "txt" }, "Reject"),
        ),
    );
}

function confirmApprovalDecision(e, notification, decision) {
    e.preventDefault();
    e.stopPropagation();

    const label = decision == "approved" ? "Approve" : "Reject";
    app.modals.confirm(
        `${label} automation approval?`,
        () => app.store.resolveNotificationApproval(notification, decision),
        null,
        { yesButton: label, noButton: "Cancel" },
    );
}

function notificationApprovalId(notification) {
    const data = notification?.data || {};
    return data.approvalId || (notification?.sourceCollection == "_approvals" ? notification?.sourceRecord : "");
}

function notificationBellLabel() {
    const count = app.store.unreadNotifications;
    if (!count) {
        return "Notifications";
    }

    return `${count} unread notification${count == 1 ? "" : "s"}`;
}

function formatNotificationCount(count) {
    if (count > 99) {
        return "99+";
    }
    if (count > 9) {
        return "9+";
    }

    return count;
}

function formatNotificationDate(value) {
    if (!value) {
        return "";
    }

    return app.utils.toLocalDatetime(value).split(" ")[0];
}
