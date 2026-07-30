import { useEffect, useRef, useState } from 'react';

// Focus containment for a modal dialog: Tab cycles inside it, and whatever opened it gets
// focus back when it closes.
//
// `role="dialog"` + `aria-modal="true"` PROMISE that the rest of the page is unreachable while
// the dialog is up. Nothing in this app made that true for the keyboard — every overlay here is
// a plain positioned <div> whose only containment is a click-outside handler, so Tab walked
// straight out of an open dialog into the page behind it. That is an accessibility defect on
// its own, and on the Defects register it was also a way to reach the bulk bar underneath an
// open editor (see utils/defectActions isBulkLocked for the write-level guard that closes the
// data-loss half of it — this half only closes the path).
//
// Deliberately minimal, and deliberately NOT an Escape handler: components/Modal already owns
// its own Escape/Enter keys and DefectModal has never closed on Escape, so adding one here
// would either double-fire or change a behaviour nothing asked to change.
//
// Returns a ref for the dialog element. Give that element tabIndex={-1} so it can hold focus
// itself while every control inside it is disabled (which is exactly what a submitting form is).

const FOCUSABLE = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])',
].join(',');

export function useDialogFocus() {
    const ref = useRef(null);

    // The opener, read DURING THE FIRST RENDER rather than in the effect below — a lazy initial
    // state is the one hook that runs before the commit.
    //
    // Both dialogs autoFocus a field, and React performs that focus while committing the tree,
    // BEFORE any effect runs — passive or layout, and regardless of where in the tree the hook
    // sits, because a parent's layout effects run after its children are mounted. Measured, not
    // reasoned about: read from an effect this recorded the dialog's own title input as the
    // "opener", so closing the dialog focused a node that had just been unmounted and focus fell
    // to <body> (e2e defects_register "closing a dialog hands focus back…" pins it). Render is
    // the last moment at which focus is still on the button that was pressed.
    const [opener] = useState(() => (typeof document === 'undefined' ? null : document.activeElement));

    // Mount/unmount only: the dialog's identity does not change while it is open, and re-running
    // this on an incidental re-render would churn focus out from under whoever is typing.
    useEffect(() => {
        const node = ref.current;
        if (!node) return undefined;

        // Only claimed if nothing inside has it already — both dialogs autoFocus a field, and
        // pulling focus onto the container would undo that.
        if (!node.contains(document.activeElement)) node.focus();

        const onKey = (event) => {
            if (event.key !== 'Tab') return;
            const items = Array.from(node.querySelectorAll(FOCUSABLE));
            const active = document.activeElement;

            // Nothing left to land on (a submitting form disables every control), so the
            // container holds it rather than letting Tab escape to the page.
            if (items.length === 0) {
                event.preventDefault();
                node.focus();
                return;
            }

            const first = items[0];
            const last = items[items.length - 1];
            if (!node.contains(active)) {
                event.preventDefault();
                (event.shiftKey ? last : first).focus();
                return;
            }
            if (event.shiftKey && (active === first || active === node)) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && active === last) {
                event.preventDefault();
                first.focus();
            }
        };

        // Capture phase: the pages below carry document-level key handlers of their own, and
        // the containment has to decide where Tab goes before any of them sees it.
        document.addEventListener('keydown', onKey, true);
        return () => {
            document.removeEventListener('keydown', onKey, true);
            // A cleanup is not the same thing as a close, and the restore must only run on a
            // close. Both conditions below were MEASURED at each of the two, not assumed:
            //
            //   · real unmount — the dialog is already detached (`isConnected` false, React
            //     removes the DOM in the mutation phase, before passive cleanups flush) and the
            //     browser has dropped focus to <body> because the node holding it went away;
            //   · StrictMode's development rehearsal — the effect is torn down and set straight
            //     back up with the dialog still on screen and still holding focus. Restoring
            //     there pulled focus off the autofocused field onto the opener, and the re-run
            //     then parked it on the container, so every dialog in the app opened with its
            //     first field unfocused.
            //
            // Requiring both is also the honest rule for a close: if anything else has claimed
            // focus by now, it wanted it more recently than this dialog did.
            if (node.isConnected) return;
            const active = document.activeElement;
            if (active && active !== document.body) return;
            // The opener can have gone with the dialog — a row deleted by the very confirmation
            // it opened — and focusing a detached node silently does nothing, which is the bug
            // this restore exists to fix rather than a behaviour to fall back on.
            if (opener && document.contains(opener)) opener.focus?.();
        };
        // `opener` is captured once at mount and never reassigned, so this stays a mount/unmount
        // effect; it is listed only because the cleanup closes over it.
    }, [opener]);

    return ref;
}

export default useDialogFocus;
