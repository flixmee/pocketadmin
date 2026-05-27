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
    const rowContent = [
        t.span({
            className: () => `notification-severity ${notification.severity || "info"}`,
            ariaHidden: true,
        }),
        t.span(
            { className: "notification-content" },
            t.span({ className: "notification-title txt-ellipsis" }, notification.title || "Untitled"),
            () => {
                if (notification.message) {
                    return t.span({ className: "notification-message txt-ellipsis" }, notification.message);
                }
            },
            t.time(
                { className: "notification-date", dateTime: notification.created },
                formatNotificationDate(notification.created),
            ),
        ),
    ];

    return t.div(
        {
            className: () => `notification-row ${notification.read ? "" : "unread"}`,
        },
        notification.actionUrl
            ? t.a(
                {
                    className: "notification-row-link",
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
            )
            : t.button(
                {
                    type: "button",
                    className: "notification-row-link",
                    onclick: () => {
                        if (!notification.read) {
                            app.store.markNotificationRead(notification.id);
                        }
                    },
                },
                rowContent,
            ),
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
