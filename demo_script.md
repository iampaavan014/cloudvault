# 🎥 CloudVault Demo Script

## 1. Introduction (30s)
"Hi, I'm Paavan, and this is **CloudVault**. It's an open-source platform that solves the #1 pain point in Kubernetes: **Storage Cost Visibility & Optimization**."

"Most tools tell you *that* you're spending money. CloudVault tells you *where* you're wasting it and *how* to fix it."

## 2. The Problem (30s)
"In a typical multi-cloud setup, you have hundreds of PVCs. Some are orphaned (Zombie volumes), some are massively over-provisioned (1TB for 5GB of data), and some are on expensive SSDs when they should be on HDD."

"Finding these manually is impossible. CloudVault automates it."

## 3. Demo: Basic Cost Visibility (1m)
"Let's look at the CLI. First, I'll run a cost accumulation report."

**Command:**
```bash
./bin/cloudvault cost
```

**Talking Points:**
- "Instantly, I see my total monthly spend."
- "I can see the breakdown by Namespace (Production is 65% of cost)."
- "I can see the cost by Storage Class."
- "And here are the Top 10 most expensive volumes."

## 4. Demo: Optimization Recommendations (1m)
"Now, let's find some savings."

**Command:**
```bash
./bin/cloudvault recommendations
```

**Talking Points:**
- "CloudVault analyzes the metadata and finds Zombie volumes (unused for 45 days)."
- "It finds oversized volumes based on heuristic checks."
- "It suggests cheaper storage classes."

## 5. Demo: Enhanced Intelligence (Prometheus) (2m)
"But metadata isn't enough. We need **real** usage data. CloudVault integrates with Prometheus to see what's actually happening inside the volumes."

"I have a mock Prometheus running to simulate real-world data."

**Command:**
```bash
./bin/cloudvault recommendations --prometheus http://localhost:9090
```

**Talking Points:**
- "Notice this new recommendation: `postgres-backup`."
- "It's a 200GB volume, but Prometheus tells us it's only using 10GB (5%)."
- "CloudVault recommends resizing it to 15GB, saving us $18.50/month on just this one volume."
- "This isn't a guess; it's data-driven optimization."

## 6. Demo: The Professional Dashboard (2m)
"Now, let's step into the CloudVault Control Plane. This is where storage intelligence meets enterprise management."

**Action:** Open browser to `http://localhost:8080`.

**Talking Points:**
- **The Sidebar**: "Notice the professional multi-view layout. We have dedicated spaces for high-level monitoring and deep-dive optimization."
- **Overview Page**: "Our monthly spend is $53.50. We can instantly see the cost distribution across namespaces."
- **Cost Analysis**: "Switching to Cost Analysis, we get high-density charts and a per-PVC inventory. I can see exactly why the `production` namespace is costing us $35.00."
- **Optimization View**: "The Optimization view focuses on action. We can filter findings by impact and apply fixes with one-click generated commands."
- **Responsiveness**: "And it's built for the modern operator. Whether you're at your desk or on your mobile, the glassmorphic UI adapts to your viewport."

## 7. Closing (30s)
"CloudVault is more than just a cost calculator; it's a complete intelligence layer for your Kubernetes storage."

"CloudVault: Stop wasting money on Kubernetes storage. Start optimizing today."
