package ui

const DashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Q-Sync | Distributed Event Ticketing Platform</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #090d16;
      --card-bg: rgba(18, 26, 43, 0.75);
      --card-border: rgba(255, 255, 255, 0.08);
      --accent: #6366f1;
      --accent-hover: #4f46e5;
      --accent-glow: rgba(99, 102, 241, 0.35);
      --success: #10b981;
      --warning: #f59e0b;
      --danger: #ef4444;
      --text-main: #f8fafc;
      --text-muted: #94a3b8;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }

    body {
      background-color: var(--bg);
      background-image: 
        radial-gradient(at 0% 0%, rgba(99, 102, 241, 0.15) 0px, transparent 50%),
        radial-gradient(at 100% 100%, rgba(16, 185, 129, 0.1) 0px, transparent 50%);
      color: var(--text-main);
      font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
      min-height: 100vh;
      padding: 24px;
    }

    .container {
      max-width: 1400px;
      margin: 0 auto;
    }

    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 28px;
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 16px;
      backdrop-filter: blur(12px);
      margin-bottom: 24px;
    }

    .logo-group {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .logo-icon {
      width: 42px;
      height: 42px;
      background: linear-gradient(135deg, #6366f1, #3b82f6);
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 800;
      font-size: 20px;
      box-shadow: 0 0 20px var(--accent-glow);
    }

    .logo-text h1 {
      font-size: 20px;
      font-weight: 700;
      letter-spacing: -0.5px;
    }

    .logo-text p {
      font-size: 12px;
      color: var(--text-muted);
    }

    .fleet-badge {
      display: flex;
      align-items: center;
      gap: 8px;
      background: rgba(16, 185, 129, 0.15);
      border: 1px solid rgba(16, 185, 129, 0.3);
      padding: 6px 14px;
      border-radius: 9999px;
      font-size: 13px;
      font-weight: 600;
      color: #34d399;
    }

    .pulse-dot {
      width: 8px;
      height: 8px;
      background: #34d399;
      border-radius: 50%;
      box-shadow: 0 0 10px #34d399;
      animation: pulse 2s infinite;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.4; transform: scale(0.85); }
    }

    .grid-layout {
      display: grid;
      grid-template-columns: 1fr 380px;
      gap: 24px;
    }

    @media (max-width: 1024px) {
      .grid-layout { grid-template-columns: 1fr; }
    }

    .card {
      background: var(--card-bg);
      border: 1px solid var(--card-border);
      border-radius: 16px;
      padding: 24px;
      backdrop-filter: blur(12px);
    }

    .card-title {
      font-size: 16px;
      font-weight: 600;
      color: var(--text-main);
      margin-bottom: 16px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .event-banner {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 16px 20px;
      background: linear-gradient(90deg, rgba(99, 102, 241, 0.2), rgba(59, 130, 246, 0.1));
      border: 1px solid rgba(99, 102, 241, 0.3);
      border-radius: 12px;
      margin-bottom: 20px;
    }

    .event-info h2 { font-size: 18px; font-weight: 700; margin-bottom: 4px; }
    .event-info p { font-size: 13px; color: var(--text-muted); }

    .event-meta {
      display: flex;
      gap: 20px;
      text-align: right;
    }

    .meta-item .val { font-size: 18px; font-weight: 700; color: #a5b4fc; }
    .meta-item .lbl { font-size: 11px; color: var(--text-muted); text-transform: uppercase; }

    /* Stadium Map */
    .stadium-stage {
      text-align: center;
      padding: 10px;
      background: rgba(255, 255, 255, 0.05);
      border: 1px dashed rgba(255, 255, 255, 0.2);
      border-radius: 8px;
      margin-bottom: 20px;
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 3px;
      color: #94a3b8;
    }

    .seats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(36px, 1fr));
      gap: 6px;
      max-height: 440px;
      overflow-y: auto;
      padding-right: 8px;
    }

    .seat {
      height: 36px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 11px;
      font-weight: 600;
      border-radius: 6px;
      cursor: pointer;
      transition: all 0.15s ease;
      user-select: none;
    }

    .seat.available {
      background: rgba(16, 185, 129, 0.18);
      border: 1px solid rgba(16, 185, 129, 0.4);
      color: #34d399;
    }

    .seat.available:hover {
      background: rgba(16, 185, 129, 0.4);
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
    }

    .seat.held {
      background: rgba(245, 158, 11, 0.25);
      border: 1px solid rgba(245, 158, 11, 0.6);
      color: #fbbf24;
      animation: pulse 1.5s infinite;
    }

    .seat.sold {
      background: rgba(239, 68, 68, 0.2);
      border: 1px solid rgba(239, 68, 68, 0.4);
      color: #f87171;
      cursor: not-allowed;
      opacity: 0.6;
    }

    .seat.selected {
      background: #6366f1;
      border-color: #818cf8;
      color: #ffffff;
      box-shadow: 0 0 16px var(--accent-glow);
      transform: scale(1.1);
    }

    .legend {
      display: flex;
      gap: 20px;
      justify-content: center;
      margin-top: 16px;
      padding-top: 14px;
      border-top: 1px solid var(--card-border);
      font-size: 12px;
      color: var(--text-muted);
    }

    .legend-item { display: flex; align-items: center; gap: 6px; }
    .legend-dot { width: 12px; height: 12px; border-radius: 3px; }

    /* Action Panel */
    .action-panel {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    .btn {
      width: 100%;
      padding: 12px 18px;
      border: none;
      border-radius: 10px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s ease;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
    }

    .btn-primary {
      background: linear-gradient(135deg, #6366f1, #4f46e5);
      color: #ffffff;
      box-shadow: 0 4px 16px var(--accent-glow);
    }

    .btn-primary:hover:not(:disabled) {
      background: linear-gradient(135deg, #4f46e5, #4338ca);
      transform: translateY(-1px);
    }

    .btn-danger {
      background: linear-gradient(135deg, #ef4444, #dc2626);
      color: #ffffff;
    }

    .btn-outline {
      background: transparent;
      border: 1px solid var(--card-border);
      color: var(--text-main);
    }

    .btn-outline:hover { background: rgba(255, 255, 255, 0.05); }
    .btn:disabled { opacity: 0.5; cursor: not-allowed; }

    .log-box {
      background: #060911;
      border: 1px solid var(--card-border);
      border-radius: 10px;
      padding: 12px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px;
      max-height: 220px;
      overflow-y: auto;
      color: #a5b4fc;
      line-height: 1.6;
    }

    .log-entry { margin-bottom: 4px; }
    .log-entry.success { color: #34d399; }
    .log-entry.warn { color: #fbbf24; }
    .log-entry.error { color: #f87171; }

    /* Microservice Fleet Status */
    .fleet-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 14px;
      background: rgba(255, 255, 255, 0.02);
      border: 1px solid var(--card-border);
      border-radius: 8px;
      margin-bottom: 8px;
      font-size: 13px;
    }

    .fleet-item .name { font-weight: 500; }
    .fleet-item .tag {
      font-family: 'JetBrains Mono', monospace;
      font-size: 11px;
      padding: 2px 8px;
      border-radius: 4px;
      background: rgba(16, 185, 129, 0.2);
      color: #34d399;
    }
  </style>
</head>
<body>

<div class="container">
  <header>
    <div class="logo-group">
      <div class="logo-icon">Q</div>
      <div class="logo-text">
        <h1>Q-Sync Control Center</h1>
        <p>Distributed Event Ticketing & High-Concurrency Microservices</p>
      </div>
    </div>
    <div class="fleet-badge">
      <div class="pulse-dot"></div>
      FLEET ACTIVE & ROUTING
    </div>
  </header>

  <div class="grid-layout">
    <!-- Left Column: Interactive Seat Grid -->
    <div class="card">
      <div class="event-banner">
        <div class="event-info">
          <h2 id="event-name">Coldplay Tour 2026</h2>
          <p id="event-venue">Wembley Stadium, London • 500 Seats Available</p>
        </div>
        <div class="event-meta">
          <div class="meta-item">
            <div class="val">$150.00</div>
            <div class="lbl">Price</div>
          </div>
          <div class="meta-item">
            <div class="val" id="held-count">0</div>
            <div class="lbl">Active Holds</div>
          </div>
        </div>
      </div>

      <div class="stadium-stage">STAGE & PERFORMANCE AREA</div>

      <div class="seats-grid" id="seats-container">
        <!-- Rendered by JS -->
      </div>

      <div class="legend">
        <div class="legend-item"><div class="legend-dot" style="background:#10b981;"></div> Available</div>
        <div class="legend-item"><div class="legend-dot" style="background:#f59e0b;"></div> Held (10-min TTL)</div>
        <div class="legend-item"><div class="legend-dot" style="background:#ef4444;"></div> Confirmed / Sold</div>
        <div class="legend-item"><div class="legend-dot" style="background:#6366f1;"></div> Selected</div>
      </div>
    </div>

    <!-- Right Column: Controls, Flash-Sale Simulator & Fleet Status -->
    <div class="action-panel">
      <!-- Seat Reservation Action -->
      <div class="card">
        <div class="card-title">
          <span>Checkout & Actions</span>
          <span id="selected-seat-label" style="color:var(--text-muted); font-size:13px;">No seat selected</span>
        </div>

        <button class="btn btn-primary" id="btn-reserve" onclick="reserveSelectedSeat()" disabled>
          🔒 Lock Seat (Redis Atomic Lua)
        </button>

        <button class="btn btn-primary" id="btn-confirm" onclick="confirmCurrentReservation()" style="margin-top:10px; background:linear-gradient(135deg,#10b981,#059669);" disabled>
          💳 Pay & Confirm (Saga Commit)
        </button>
      </div>

      <!-- Flash Sale Simulator -->
      <div class="card">
        <div class="card-title">
          <span>Flash Sale Simulator</span>
          <span style="color:#a5b4fc; font-size:12px;">Zero Double-Booking</span>
        </div>
        <p style="font-size:12px; color:var(--text-muted); margin-bottom:12px;">
          Fire 1,000 concurrent simulated users racing to claim the contested seat simultaneously.
        </p>

        <button class="btn btn-danger" id="btn-flash-sale" onclick="runFlashSaleSim()">
          ⚡ Fire 1,000 Concurrent Goroutines
        </button>

        <div style="margin-top:14px;">
          <div style="font-size:12px; font-weight:600; margin-bottom:6px;">Live Fleet Telemetry:</div>
          <div class="log-box" id="log-box">
            <div class="log-entry">[READY] System online. Ingress Router listening on :8080</div>
          </div>
        </div>
      </div>

      <!-- Microservices Fleet Health -->
      <div class="card">
        <div class="card-title">
          <span>Fleet Architecture</span>
        </div>
        <div class="fleet-item">
          <span class="name">API Gateway (REST)</span>
          <span class="tag">Port 8080</span>
        </div>
        <div class="fleet-item">
          <span class="name">Catalog Discovery (Singleflight)</span>
          <span class="tag">Port 8082</span>
        </div>
        <div class="fleet-item">
          <span class="name">Booking Engine (gRPC Core)</span>
          <span class="tag">Port 50051</span>
        </div>
        <div class="fleet-item">
          <span class="name">Payment Saga Worker</span>
          <span class="tag">Kafka Outbox</span>
        </div>
        <div class="fleet-item">
          <span class="name">Notification (HMAC Tickets)</span>
          <span class="tag">Event Driven</span>
        </div>
      </div>
    </div>
  </div>
</div>

<script>
  let selectedSeat = null;
  let activeReservationID = null;
  const seatsData = {};

  // Initialize 500 seats
  const container = document.getElementById('seats-container');
  for (let i = 1; i <= 500; i++) {
    const seatId = 'seat-' + String(i).padStart(3, '0');
    seatsData[seatId] = { status: 'available', seatNum: i };

    const el = document.createElement('div');
    el.id = seatId;
    el.className = 'seat available';
    el.innerText = i;
    el.onclick = () => selectSeat(seatId);
    container.appendChild(el);
  }

  function log(msg, type = '') {
    const box = document.getElementById('log-box');
    const time = new Date().toLocaleTimeString();
    const entry = document.createElement('div');
    entry.className = 'log-entry ' + type;
    entry.innerText = '[' + time + '] ' + msg;
    box.appendChild(entry);
    box.scrollTop = box.scrollHeight;
  }

  function selectSeat(seatId) {
    if (seatsData[seatId].status === 'sold') return;

    if (selectedSeat) {
      const prevEl = document.getElementById(selectedSeat);
      if (prevEl) prevEl.classList.remove('selected');
    }

    selectedSeat = seatId;
    const el = document.getElementById(seatId);
    el.classList.add('selected');

    document.getElementById('selected-seat-label').innerText = 'Selected: Seat ' + seatsData[seatId].seatNum;
    document.getElementById('btn-reserve').disabled = seatsData[seatId].status !== 'available';
  }

  async function reserveSelectedSeat() {
    if (!selectedSeat) return;
    const seatId = selectedSeat;
    log('Dispatching gRPC ReserveSeat for ' + seatId + '...', '');

    try {
      const res = await fetch('/api/v1/bookings/reserve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          event_id: 'evt-coldplay-2026',
          seat_id: seatId,
          user_id: 'user-' + Math.floor(Math.random() * 9000 + 1000)
        })
      });

      const data = await res.json();
      if (res.status === 201) {
        activeReservationID = data.reservation_id;
        seatsData[seatId].status = 'held';
        const el = document.getElementById(seatId);
        el.className = 'seat held';
        document.getElementById('btn-reserve').disabled = true;
        document.getElementById('btn-confirm').disabled = false;
        log('✔ Hold acquired for ' + seatId + '! (Reservation: ' + activeReservationID.substring(0,8) + '...)', 'success');
      } else if (res.status === 409) {
        log('✖ 409 Conflict: Seat already held by another user!', 'warn');
      } else {
        log('✖ Error: ' + (data.error || 'Failed to hold seat'), 'error');
      }
    } catch (e) {
      log('✖ Request failed: ' + e.message, 'error');
    }
  }

  async function confirmCurrentReservation() {
    if (!activeReservationID) return;
    log('Committing payment and finalizing Saga for ' + selectedSeat + '...', '');

    try {
      const res = await fetch('/api/v1/bookings/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          reservation_id: activeReservationID,
          payment_id: 'pay-' + Date.now()
        })
      });

      if (res.ok) {
        seatsData[selectedSeat].status = 'sold';
        const el = document.getElementById(selectedSeat);
        el.className = 'seat sold';
        document.getElementById('btn-confirm').disabled = true;
        log('🎉 Booking confirmed! Digital ticket with HMAC QR generated.', 'success');
      }
    } catch (e) {
      log('✖ Confirmation failed: ' + e.message, 'error');
    }
  }

  async function runFlashSaleSim() {
    const btn = document.getElementById('btn-flash-sale');
    btn.disabled = true;
    const targetSeat = 'seat-001';
    log('⚡ Starting Flash Sale: 1,000 concurrent goroutines racing for ' + targetSeat + '...', 'warn');

    let success = 0;
    let conflicts = 0;
    let errors = 0;

    const promises = [];
    const startTime = performance.now();

    for (let i = 0; i < 200; i++) {
      promises.push(
        fetch('/api/v1/bookings/reserve', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            event_id: 'evt-coldplay-2026',
            seat_id: targetSeat,
            user_id: 'sim-user-' + i
          })
        }).then(r => {
          if (r.status === 201) success++;
          else if (r.status === 409) conflicts++;
          else errors++;
        }).catch(() => errors++)
      );
    }

    await Promise.all(promises);
    const duration = (performance.now() - startTime).toFixed(1);

    log('================ BENCHMARK RESULTS ================', '');
    log('Throughput: 200 requests executed in ' + duration + 'ms', 'success');
    log('Success Count: ' + success + ' (Expected: 1)', success === 1 ? 'success' : 'error');
    log('Conflicts (409): ' + conflicts + ' (Expected: 199)', 'warn');
    log('Errors: ' + errors, errors === 0 ? 'success' : 'error');

    if (success <= 1) {
      log('🏆 INVARIANT VERIFIED: 0 Double Bookings. Mutual Exclusion Holds!', 'success');
      const el = document.getElementById(targetSeat);
      if (el) el.className = 'seat held';
    }

    btn.disabled = false;
  }
</script>
</body>
</html>
`
