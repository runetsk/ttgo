import RunTimeline from '../../components/RunTimeline';

export default function TimelineTab({ run, onNavigateToResult }) {
    return (
        <RunTimeline
            results={run.run_results}
            onNavigateToResult={onNavigateToResult}
        />
    );
}
