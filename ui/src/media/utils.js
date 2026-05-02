function isAbsoluteURL(value) {
    return /^[a-z][a-z\d+.-]*:\/\//i.test((value || "").trim());
}

function parseMediaFileURL(value) {
    value = (value || "").trim();
    if (!value) {
        return null;
    }

    try {
        const url = new URL(value, window.location.origin);
        const parts = url.pathname.split("/").filter(Boolean);
        if (
            parts.length !== 5
            || parts[0] !== "api"
            || parts[1] !== "files"
        ) {
            return null;
        }

        return {
            url,
            collection: decodeURIComponent(parts[2]),
            recordId: decodeURIComponent(parts[3]),
            filename: decodeURIComponent(parts[4]),
            isAbsolute: isAbsoluteURL(value),
        };
    } catch {
        return null;
    }
}

export function isMediaFileURL(value) {
    return !!parseMediaFileURL(value);
}

export function normalizeMediaFileURL(value) {
    const parsed = parseMediaFileURL(value);
    if (!parsed) {
        return (value || "").trim();
    }

    parsed.url.search = "";
    parsed.url.hash = "";

    return parsed.isAbsolute
        ? parsed.url.toString()
        : parsed.url.pathname;
}

export function getMediaFileURL(record) {
    if (record?.kind !== "file" || !record?.file) {
        return "";
    }

    return normalizeMediaFileURL(app.pb.files.getURL(record, record.file));
}

export function getMediaSelectionLabel(value) {
    const parsed = parseMediaFileURL(value);
    if (parsed?.filename) {
        return parsed.filename;
    }

    value = (value || "").trim();
    if (!value) {
        return "";
    }

    try {
        return decodeURIComponent(value.split("/").pop() || value);
    } catch {
        return value.split("/").pop() || value;
    }
}

export function buildMediaThumbURL(value, { width = 50, height = 50 } = {}) {
    const parsed = parseMediaFileURL(value);
    if (!parsed) {
        return (value || "").trim();
    }

    parsed.url.searchParams.set("thumb", `${width}x${height}`);

    return parsed.isAbsolute
        ? parsed.url.toString()
        : `${parsed.url.pathname}${parsed.url.search}`;
}
