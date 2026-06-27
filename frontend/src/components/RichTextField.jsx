import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { useState, useEffect, useRef, useCallback } from 'react'
import SafeHTML from './shared/SafeHTML'

/**
 * RichTextField — a click-to-expand inline rich text editor.
 *
 * Read-only mode (default):
 *   Renders `value` as HTML via dangerouslySetInnerHTML. Safe because
 *   the backend sanitizes all HTML before storage (FR-002). This mode
 *   preserves ALL HTML tags including those unsupported by the editor
 *   (e.g. <table>, <img>) — satisfying FR-001 and FR-005 for display.
 *
 * Edit mode (activated on click):
 *   TipTap editor with StarterKit — supports bold, italic, paragraph,
 *   hard break, bullet list, ordered list (FR-003). Calling onChange on
 *   every content change; empty editor returns "" not "<p></p>" (FR-007).
 *
 * Props:
 *   value      {string}   Current HTML string (or empty string).
 *   onChange   {function} Called with new HTML string on every content change.
 *   placeholder {string}  Text shown when field is empty and not editing.
 *   readOnly   {boolean}  If true, disables click-to-expand (optional).
 */
export default function RichTextField({ value, onChange, placeholder, readOnly = false }) {
  const [isEditing, setIsEditing] = useState(false)

  // Keep a ref to onChange so TipTap's onUpdate closure is never stale.
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange

  // Track the last HTML we pushed to the parent so we can tell if an
  // incoming `value` change is external (API reload) vs our own echo.
  const lastEmittedRef = useRef(value ?? '')

  const editor = useEditor({
    extensions: [StarterKit],
    content: value || '',
    editable: true,
    onUpdate: ({ editor }) => {
      const html = editor.isEmpty ? '' : editor.getHTML()
      lastEmittedRef.current = html
      onChangeRef.current(html)
    },
  })

  // Sync external value changes (e.g. test reload from API) into the editor.
  // Only runs when NOT editing to avoid clobbering in-progress user input.
  useEffect(() => {
    if (!editor || isEditing) return
    const incoming = value ?? ''
    if (incoming !== lastEmittedRef.current) {
      editor.commands.setContent(incoming, false)
      lastEmittedRef.current = incoming
    }
  }, [value, editor, isEditing])

  // Activate edit mode: load current value and focus.
  const activate = useCallback(() => {
    if (readOnly || isEditing || !editor) return
    // Sync the latest parent value into the editor before showing it.
    const incoming = value ?? ''
    editor.commands.setContent(incoming, false)
    lastEmittedRef.current = incoming
    setIsEditing(true)
    // Focus after React re-render puts the editor into view.
    setTimeout(() => editor.commands.focus('end'), 0)
  }, [readOnly, isEditing, editor, value])

  // Deactivate edit mode when focus leaves the entire container.
  const handleBlur = useCallback((e) => {
    // relatedTarget is the element receiving focus; if it's inside this
    // container (e.g. a toolbar button), don't deactivate.
    if (e.currentTarget.contains(e.relatedTarget)) return
    setIsEditing(false)
  }, [])

  // Toolbar button helper — uses onMouseDown to keep editor focus.
  const ToolbarButton = ({ onAction, title, children, isActive }) => (
    <button
      type="button"
      className={`rich-text-toolbar-btn${isActive ? ' is-active' : ''}`}
      onMouseDown={(e) => {
        e.preventDefault()   // prevent blur before action fires
        onAction()
      }}
      title={title}
    >
      {children}
    </button>
  )

  const isEmpty = !value || value.trim() === '' || value === '<p></p>'

  return (
    <div
      className={`rich-text-field${isEditing ? ' is-editing' : ''}${readOnly ? ' is-readonly' : ''}`}
      onClick={activate}
      onBlur={handleBlur}
      tabIndex={readOnly ? undefined : 0}
      role={readOnly ? undefined : 'button'}
      aria-label={readOnly ? undefined : 'Click to edit'}
    >
      {/* Formatting toolbar — only shown when editing */}
      {isEditing && editor && (
        <div className="rich-text-toolbar" onMouseDown={(e) => e.preventDefault()}>
          <ToolbarButton
            onAction={() => editor.chain().focus().toggleBold().run()}
            title="Bold"
            isActive={editor.isActive('bold')}
          >
            <strong>B</strong>
          </ToolbarButton>
          <ToolbarButton
            onAction={() => editor.chain().focus().toggleItalic().run()}
            title="Italic"
            isActive={editor.isActive('italic')}
          >
            <em>I</em>
          </ToolbarButton>
          <span className="rich-text-toolbar-separator" />
          <ToolbarButton
            onAction={() => editor.chain().focus().toggleBulletList().run()}
            title="Bullet list"
            isActive={editor.isActive('bulletList')}
          >
            •≡
          </ToolbarButton>
          <ToolbarButton
            onAction={() => editor.chain().focus().toggleOrderedList().run()}
            title="Ordered list"
            isActive={editor.isActive('orderedList')}
          >
            1.
          </ToolbarButton>
          <span className="rich-text-toolbar-separator" />
          <ToolbarButton
            onAction={() => editor.chain().focus().setHardBreak().run()}
            title="Line break (Shift+Enter)"
          >
            ↵
          </ToolbarButton>
        </div>
      )}

      {/* Read-only display — renders full HTML via browser (all tags preserved) */}
      {!isEditing && (
        <SafeHTML
          className={`rich-text-display${isEmpty ? ' is-empty' : ''}`}
          html={isEmpty ? '' : (value || '')}
        />
      )}
      {!isEditing && isEmpty && placeholder && (
        <div className="rich-text-placeholder">{placeholder}</div>
      )}

      {/* Edit mode — TipTap editor (hidden while not editing to avoid layout shifts) */}
      <div style={{ display: isEditing ? 'block' : 'none' }}>
        <EditorContent editor={editor} className="rich-text-editor-content" />
      </div>
    </div>
  )
}
