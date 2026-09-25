import React, { useEffect, useMemo, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { diagramModel } from '../../utils/analysisFlow';
import AnalysisFlowDiagram from './AnalysisFlowDiagram';
import TypeSafeSettingsCard from './TypeSafeSettingsCard';
import AIFailureAnalysisSettings from '../AIFailureAnalysisSettings';

// FailureAnalysisSection is the Failure analysis tab of Settings → AI: the process diagram on top,
// then the TypeSafe.ai and AI Failure Analysis cards. The cards keep their own loading, validation
// and saving and report their state here, so the diagram follows unsaved edits and the tab can
// show an unsaved-changes dot (onDirtyChange).
export default function FailureAnalysisSection({ isAdmin, onDirtyChange }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [trigger, setTrigger] = useState('manual');
    const [typesafeCard, setTypesafeCard] = useState({ status: 'loading' });
    const [faCard, setFaCard] = useState({ status: 'loading' });

    const model = useMemo(() => diagramModel({
        aiEnabled: aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger,
    }), [aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger]);

    const dirty = model.state === 'ready' && !!model.dirty;
    useEffect(() => { onDirtyChange?.(dirty); }, [dirty, onDirtyChange]);

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }} data-testid="failure-analysis-section">
            <AnalysisFlowDiagram model={model} trigger={trigger} onTriggerChange={setTrigger} />
            <TypeSafeSettingsCard isAdmin={isAdmin} onStateChange={setTypesafeCard} />
            <AIFailureAnalysisSettings isAdmin={isAdmin} onStateChange={setFaCard} />
        </div>
    );
}
