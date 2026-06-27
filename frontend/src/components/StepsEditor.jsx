import React from 'react';
import RichTextField from './RichTextField';
import {
    DndContext,
    closestCenter,
    KeyboardSensor,
    PointerSensor,
    useSensor,
    useSensors,
} from '@dnd-kit/core';
import {
    arrayMove,
    SortableContext,
    sortableKeyboardCoordinates,
    verticalListSortingStrategy,
    useSortable,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';

function SortableItem(props) {
    const {
        attributes,
        listeners,
        setNodeRef,
        transform,
        transition,
    } = useSortable({ id: props.id });

    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
        display: 'flex',
        alignItems: 'center',
        padding: '8px',
        background: 'var(--bg-tertiary)',
        marginBottom: '8px',
        borderRadius: '6px',
        border: '1px solid var(--border-color)',
        gap: '12px'
    };

    return (
        <div ref={setNodeRef} style={style}>
            <div {...attributes} {...listeners} style={{ cursor: 'grab', fontSize: '1.2rem', color: 'var(--text-secondary)' }}>
                ☰
            </div>
            <span style={{
                minWidth: 22, height: 22, borderRadius: '50%',
                background: 'var(--bg-primary)', border: '1px solid var(--border-color)',
                color: 'var(--text-secondary)', fontSize: '0.7rem', fontWeight: 600,
                display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
            }}>
                {props.index + 1}
            </span>
            <div style={{ flex: 1, display: 'flex', gap: '12px', alignItems: 'center' }}>
                <div style={{ flex: 1 }} data-testid={`step-action-${props.step.order_index}`}>
                    <RichTextField
                        value={props.step.action}
                        onChange={(val) => props.onChange(props.step.id, 'action', val)}
                        placeholder="Action"
                    />
                </div>
                <div style={{ flex: 1 }} data-testid={`step-expected-${props.step.order_index}`}>
                    <RichTextField
                        value={props.step.expected_result}
                        onChange={(val) => props.onChange(props.step.id, 'expected_result', val)}
                        placeholder="Expected Result"
                    />
                </div>
            </div>
            <button
                className="action-btn"
                onClick={() => props.onRemove(props.step.id)}
                style={{ color: 'var(--accent-red)' }}
                title="Delete Step"
                data-testid={`delete-step-${props.step.order_index}`}
            >
                ✕
            </button>
        </div>
    );
}

export default function StepsEditor({ steps, onChange }) {
    const sensors = useSensors(
        useSensor(PointerSensor),
        useSensor(KeyboardSensor, {
            coordinateGetter: sortableKeyboardCoordinates,
        })
    );

    const handleDragEnd = (event) => {
        const { active, over } = event;

        if (active.id !== over.id) {
            const oldIndex = steps.findIndex((step) => step.id === active.id);
            const newIndex = steps.findIndex((step) => step.id === over.id);

            const newSteps = arrayMove(steps, oldIndex, newIndex);
            // Update order index locally
            newSteps.forEach((s, idx) => s.order_index = idx);
            onChange(newSteps);
        }
    };

    const handleStepChange = (id, field, value) => {
        const newSteps = steps.map(s => {
            if (s.id === id) return { ...s, [field]: value };
            return s;
        });
        onChange(newSteps);
    };

    const removeStep = (id) => {
        onChange(steps.filter(s => s.id !== id));
    };

    const addStep = () => {
        // Use temp ID for UI until saved
        const newStep = {
            id: `temp-${Date.now()}`,
            action: '',
            expected_result: '',
            order_index: steps.length
        };
        onChange([...steps, newStep]);
    };

    return (
        <div className="steps-editor">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <h4 style={{ margin: 0 }}>Test Steps</h4>
                <button className="primary-btn" onClick={addStep} style={{ padding: '4px 12px', fontSize: '0.85rem' }} data-testid="add-step-button">+ Add Step</button>
            </div>

            <DndContext
                sensors={sensors}
                collisionDetection={closestCenter}
                onDragEnd={handleDragEnd}
            >
                <SortableContext
                    items={steps.map(s => s.id)}
                    strategy={verticalListSortingStrategy}
                >
                    {steps.map((step, index) => (
                        <SortableItem
                            key={step.id}
                            id={step.id}
                            step={step}
                            index={index}
                            onChange={handleStepChange}
                            onRemove={removeStep}
                            data-testid={`step-row-${step.order_index}`}
                        />
                    ))}
                </SortableContext>
            </DndContext>
            {steps.length === 0 && (
                <div style={{ textAlign: 'center', color: 'var(--text-secondary)', padding: 20, border: '1px dashed var(--border-color)', borderRadius: 6 }}>
                    No steps defined. Click "Add Step" to start.
                </div>
            )}
        </div>
    );
}
