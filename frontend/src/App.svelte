<script lang="ts">
  import { onMount } from 'svelte';
  import { ListProfiles, Processing } from '../wailsjs/go/main/App';

  interface EC2Instance {
    name: string;
    ami: string;
  }

  interface AWSResult {
    parameters: string[];
    instances: EC2Instance[];
  }

  let profiles: string[] = [];
  let selectedProfile: string = "";
  let filter: string = "";
  let result: AWSResult | null = null;
  let loading = false;
  let error: string | null = null;
  let feedbackMessage: string | null = null;

  onMount(async () => {
    try {
      profiles = await ListProfiles();
      if (profiles.length > 0) {
        selectedProfile = profiles[0];
      }
    } catch (err) {
      error = "Failed to load profiles: " + err;
    }
  });

  async function startProcessing() {
    if (!selectedProfile) return;
    loading = true;
    error = null;
    result = null;
    feedbackMessage = "Processing... this may take a moment if SSO login is required.";

    try {
      const res = await Processing(selectedProfile, filter);
      result = res;
      feedbackMessage = null;
    } catch (err: any) {
      error = "Error during processing: " + err;
      feedbackMessage = null;
    } finally {
      loading = false;
    }
  }
</script>

<main>
  <h1>Go Check AMI</h1>

  <div class="controls">
    <div class="control-group">
      <label for="profile">AWS Profile:</label>
      <select id="profile" bind:value={selectedProfile} disabled={loading || profiles.length === 0}>
        {#each profiles as profile}
          <option value={profile}>{profile}</option>
        {/each}
      </select>
    </div>

    <div class="control-group">
      <label for="filter">Parameter Filter:</label>
      <input 
        id="filter" 
        type="text" 
        bind:value={filter} 
        placeholder="e.g. /app/prod/ or service-name"
        disabled={loading}
      />
    </div>

    <button on:click={startProcessing} disabled={loading || !selectedProfile}>
      {loading ? 'Processing...' : 'Start'}
    </button>
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if feedbackMessage}
    <div class="feedback">{feedbackMessage}</div>
  {/if}

  {#if result}
    <div class="results">
      <div class="section">
        <h2>Parameters Found</h2>
        {#if result.parameters && result.parameters.length > 0}
          <ul class="param-list">
            {#each result.parameters as param}
              <li>{param}</li>
            {/each}
          </ul>
        {:else}
          <p>No parameters found.</p>
        {/if}
      </div>

      <div class="section">
        <h2>EC2 Instances</h2>
        {#if result.instances && result.instances.length > 0}
          <table class="ec2-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>AMI</th>
              </tr>
            </thead>
            <tbody>
              {#each result.instances as instance}
                <tr>
                  <td>{instance.name || '-'}</td>
                  <td>{instance.ami}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {:else}
          <p>No instances found.</p>
        {/if}
      </div>
    </div>
  {/if}
</main>

<style>
  main {
    padding: 20px;
    max-width: 800px;
    margin: 0 auto;
  }

  h1 {
    color: #00d1b2;
    margin-bottom: 2rem;
  }

  .controls {
    display: flex;
    flex-direction: column;
    gap: 15px;
    background: rgba(255, 255, 255, 0.05);
    padding: 20px;
    border-radius: 8px;
    margin-bottom: 20px;
  }

  .control-group {
    display: flex;
    flex-direction: column;
    text-align: left;
  }

  label {
    margin-bottom: 5px;
    font-weight: bold;
    color: #ddd;
  }

  select, input {
    padding: 10px;
    border-radius: 5px;
    border: 1px solid #444;
    background: #2a2a2a;
    color: white;
    font-size: 1rem;
  }

  button {
    padding: 12px;
    border: none;
    border-radius: 5px;
    background-color: #00d1b2;
    color: white;
    font-weight: bold;
    font-size: 1.1rem;
    cursor: pointer;
    transition: background-color 0.2s;
    margin-top: 10px;
  }

  button:hover:not(:disabled) {
    background-color: #00b89c;
  }

  button:disabled {
    background-color: #555;
    cursor: not-allowed;
    opacity: 0.7;
  }

  .error {
    color: #ff3860;
    margin-bottom: 15px;
    padding: 10px;
    background: rgba(255, 56, 96, 0.1);
    border-radius: 5px;
  }

  .feedback {
    color: #3298dc;
    margin-bottom: 15px;
  }

  .results {
    display: flex;
    flex-direction: column;
    gap: 20px;
    text-align: left;
  }

  .section {
    background: rgba(255, 255, 255, 0.05);
    padding: 15px;
    border-radius: 8px;
  }

  h2 {
    margin-top: 0;
    border-bottom: 1px solid #444;
    padding-bottom: 10px;
    font-size: 1.2rem;
    color: #ffdd57;
  }

  .param-list {
    list-style-type: none;
    padding: 0;
    max-height: 200px;
    overflow-y: auto;
  }

  .param-list li {
    padding: 5px 0;
    border-bottom: 1px solid #333;
    font-family: monospace;
  }

  .ec2-table {
    width: 100%;
    border-collapse: collapse;
  }

  .ec2-table th, .ec2-table td {
    padding: 10px;
    border-bottom: 1px solid #333;
    text-align: left;
  }

  .ec2-table th {
    color: #aaa;
  }
</style>
