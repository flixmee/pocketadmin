import { input } from "./input";
import { settings } from "./settings";
import { view } from "./view";

import { buildMediaThumbURL, isMediaFileURL } from "@/media/utils";

window.app = window.app || {};
window.app.fieldTypes = window.app.fieldTypes || {};
window.app.fieldTypes.media = {
  icon: "ri-folder-image-line",
  label: "Media",
  settings,
  input,
  view,
  summaryPriority: -1,
  filterModifiers: (f) => {
    return f.maxSelect > 1 ? ["each", "length"] : [];
  },
  dummyData: (f) => {
    return f.maxSelect > 1
      ? ["/media/hero.png", "/media/guide.pdf"]
      : "/media/hero.png";
  },
};

/**
 * Creates a file thumb element.
 *
 * @example
 * ```js
 * app.components.fileThumb(filePath, { width: 100, height: 100 })
 * ```
 *
 * @param  {Object} propsArg
 * @return {Element}
 */
window.app.components.fileThumb = function (filePath, propsArg) {
  const props = {
    width: 100,
    height: 100,
    ...propsArg,
  };

  const fileType = app.utils.getFileType(filePath);

  function resolveImageSrc() {
    if (isMediaFileURL(filePath)) {
      return buildMediaThumbURL(filePath, {
        width: props.width,
        height: props.height,
      });
    }

    if (/^[a-z][a-z\d+.-]*:\/\//i.test(filePath || "")) {
      return filePath;
    }

    return app.pb.buildURL(filePath);
  }

  const id = `file-thumb-${Math.random().toString(36).substr(2, 9)}`;
  const filename = filePath.split("/").slice(-1)[0];

  const data = store({
    isPreviewLoading: false,
    previewToken: "",
    get fileType() {
      return fileType;
    },
    get hasPreview() {
      return (
        ["image", "audio", "video"].includes(data.fileType) ||
        filePath.endsWith(".pdf")
      );
    },
    previewURL: filePath,
  });

  return t.button(
    {
      id: () => id,
      type: "button",
      draggable: false,
      className: () => `thumb sm`,
      title: () => "Preview " + filename,
      onclick: async (e) => {
        e.stopPropagation();

        if (data.hasPreview) {
          app.modals.openFilePreview(filePath);
        } else {
          const url = await resolveURL();
          app.utils.download(url, props.filename);
        }
      },
    },
    () => {
      if (fileType == "image") {
        const img = t.img({
          draggable: false,
          alt: () => "Thumb of " + filename,
          src: () => data.previewURL,
          onerror: (err) => {
            console.warn("[fileThumb] load err:", err);
            data.isPreviewLoading = false;
          },
          onload: () => {
            data.isPreviewLoading = false;
          },
          onmount: (el) => {
            data.isPreviewLoading = true;
          },
          onunmount: (el) => {
            data.isPreviewLoading = false;
          },
        });

        return img;
      }

      const iconClass = app.utils.getFileIconClass(filePath);

      return t.div(
        { className: "file-thumb" },
        t.i({ className: iconClass, ariaHidden: true }),
      );
    },
  );
};
