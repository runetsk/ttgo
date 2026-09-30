import React, { useMemo, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { diagramModel } from '../../utils/analysisFlow';
import AnalysisFlowDiagram from './AnalysisFlowDiagram';
import AIFailureAnalysisSettings from '../AIFailureAnalysisSettings';

// FailureAnalysisSection is the Failure analysis tab of Settings → AI: the process diagram on top,
// then the AI Failure Analysis card. The card keeps its own loading, validation and saving
// (registering with the page save bar) and reports its state here, so the diagram follows unsaved
// edits. The TypeSafe.ai card lives on its own tab; AISettingsPage passes its state in as
// `typesafeCard` (every tab stays mounted, so its drafts reach the diagram too).
export default function FailureAnalysisSection({ isAdmin, typesafeCard }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [trigger, setTrigger] = useState('manual');
    const [faCard, setFaCard] = useState({ status: 'loading' });

    const model = useMemo(() => diagramModel({
        aiEnabled: aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger,
    }), [aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger]);

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }} data-testid="failure-analysis-section">
            <AnalysisFlowDiagram model={model} trigger={trigger} onTriggerChange={setTrigger} />
            <AIFailureAnalysisSettings isAdmin={isAdmin} onStateChange={setFaCard} />
        </div>
    );
}
