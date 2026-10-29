<script lang="ts">
	import { onDestroy } from 'svelte';

	let {
		label,
		confirmLabel,
		disabled = false,
		onconfirm
	}: {
		label: string;
		confirmLabel: string;
		disabled?: boolean;
		onconfirm: () => void | Promise<void>;
	} = $props();

	const ARM_MS = 4000;
	let armed = $state(false);
	let pending = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;
	onDestroy(() => clearTimeout(timer));

	function disarm(): void {
		clearTimeout(timer);
		armed = false;
	}

	async function click(): Promise<void> {
		if (disabled || pending) return;
		if (!armed) {
			armed = true;
			timer = setTimeout(disarm, ARM_MS);
			return;
		}
		disarm();
		pending = true;
		try {
			await onconfirm();
		} finally {
			pending = false;
		}
	}

	function onkeydown(e: KeyboardEvent): void {
		if (!armed || e.key !== 'Escape') return;
		e.stopPropagation();
		disarm();
	}
</script>

<button
	type="button"
	class:armed
	disabled={disabled || pending}
	aria-busy={pending}
	onclick={click}
	onblur={disarm}
	{onkeydown}
>
	{pending ? '…' : armed ? confirmLabel : label}
</button>

<style>
	button {
		border: 1px solid var(--border);
		border-radius: 5px;
		padding: 3px 7px;
		background: var(--surface-2);
		color: var(--t2);
		font: inherit;
		font-size: 11px;
		cursor: pointer;
		white-space: nowrap;
	}
	button:hover:not(:disabled),
	button.armed {
		border-color: var(--green);
		background: var(--green-soft);
		color: var(--green);
	}
	button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
