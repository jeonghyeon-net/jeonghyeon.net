import { useState, useRef, useEffect } from "react";

type NewPostDialogProps = {
  onConfirm: (slug: string) => void;
  onCancel: () => void;
};

function sanitizeSlug(input: string): string {
  return input
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "-")
    .replace(/[^a-z0-9가-힣-]/g, "")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

export default function NewPostDialog({ onConfirm, onCancel }: NewPostDialogProps) {
  const [slug, setSlug] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleSubmit = () => {
    const sanitized = sanitizeSlug(slug);
    if (sanitized) {
      onConfirm(sanitized);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleSubmit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  };

  return (
    <div className="dialog-overlay" onClick={onCancel}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-title">New Post</div>
        <div className="dialog-body">
          <label className="dialog-label">Slug</label>
          <input
            ref={inputRef}
            className="dialog-input"
            type="text"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="my-new-post"
          />
          {slug && (
            <div className="dialog-preview">
              content/posts/{sanitizeSlug(slug)}/index.md
            </div>
          )}
        </div>
        <div className="dialog-actions">
          <button className="dialog-btn cancel" onClick={onCancel}>
            Cancel
          </button>
          <button
            className="dialog-btn confirm"
            onClick={handleSubmit}
            disabled={!sanitizeSlug(slug)}
          >
            Create
          </button>
        </div>
      </div>
    </div>
  );
}
