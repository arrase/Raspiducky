---
hide:
  - toc
---

<div class="hero-section">
  <div class="hero-eyebrow">
    <i class="fa-solid fa-microchip" aria-hidden="true"></i>
    <span>USB Gadget &amp; HID Appliance</span>
  </div>
  <h1>Raspiducky</h1>
  <p class="hero-subtitle">A modern, high-performance USB Gadget and DuckyScript/HIDScript execution appliance for Raspberry Pi and Linux SBCs, featuring zero-reboot ConfigFS orchestration, dual scripting engines, and an embedded web dashboard.</p>
  <div class="hero-badges">
    <img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&amp;logo=go" alt="Go Version">
    <img src="https://img.shields.io/badge/Linux-ConfigFS-FCC624?style=flat&amp;logo=linux" alt="Linux Kernel">
    <img src="https://img.shields.io/badge/USB-Gadget_API-412991?style=flat&amp;logo=usb" alt="USB Gadget">
    <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License MIT">
  </div>
  <div class="hero-badges">
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/quality_gate/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Quality Gate Status"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/coverage/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Coverage"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/bugs/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Bugs"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/vulnerabilities/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Vulnerabilities"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/code_smells/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Code Smells"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/tech_debt/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Tech Debt"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/duplicated_lines_density/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Duplicated Lines"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/security_rating/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Security Rating"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/reliability_rating/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Reliability Rating"></a>
    <a href="https://sonarcloud.io/summary/new_code?id=arrase_Raspiducky"><img src="https://img.shields.io/sonar/sqale_rating/arrase_Raspiducky?server=https%3A%2F%2Fsonarcloud.io&amp;logo=sonarcloud" alt="Maintainability Rating"></a>
  </div>
</div>

<div class="section-header">
  <h2>Key Features</h2>
  <span>Core capabilities &amp; hardware integration</span>
</div>

<div class="projects-grid">
  <div class="feature-card">
    <i class="fa-solid fa-cube feature-icon" aria-hidden="true"></i>
    <h3>Single Binary Architecture</h3>
    <p>Built in modern Golang with zero runtime external dependencies and an embedded Web UI via <code>go:embed</code>.</p>
  </div>

  <div class="feature-card">
    <i class="fa-solid fa-microchip feature-icon" aria-hidden="true"></i>
    <h3>Linux Kernel ConfigFS</h3>
    <p>Direct interaction with <code>/sys/kernel/config/usb_gadget</code> and <code>/dev/hidg*</code> char devices for dynamic gadget configuration without reboots.</p>
  </div>

  <div class="feature-card">
    <i class="fa-solid fa-code feature-icon" aria-hidden="true"></i>
    <h3>Dual Scripting Engine</h3>
    <p>Native execution of standard Hak5 Rubber Ducky scripts (<code>.txt</code>) and advanced ECMAScript 5.1+ JavaScript payloads (<code>.js</code>).</p>
  </div>

  <div class="feature-card">
    <i class="fa-solid fa-lightbulb feature-icon" aria-hidden="true"></i>
    <h3>Host LED Listener</h3>
    <p>Monitors output reports from <code>/dev/hidg0</code> (<code>NUMLOCK</code>, <code>CAPSLOCK</code>, <code>SCROLLLOCK</code>) enabling target-host synchronization and trigger-based execution.</p>
  </div>

  <div class="feature-card">
    <i class="fa-solid fa-gauge-high feature-icon" aria-hidden="true"></i>
    <h3>Web Dashboard &amp; REST API</h3>
    <p>Modern dark-mode SPA with real-time WebSocket log streaming, script editor, job manager, and live gadget controls.</p>
  </div>

  <div class="feature-card">
    <i class="fa-solid fa-layer-group feature-icon" aria-hidden="true"></i>
    <h3>Multi-Board Support</h3>
    <p>Cross-compiled binaries for ARMv6, ARMv7, ARM64, and AMD64 targeting Raspberry Pi Zero, Pi 3, Pi 4, Pi 5, Orange Pi, and NanoPi.</p>
  </div>
</div>

<div class="section-header">
  <h2>Screenshot Gallery</h2>
  <span>Embedded dark-mode SPA &amp; live console</span>
</div>

| Main Dashboard & Gadget Control | Script Editor & Live Execution Console | Payload Library & Job Manager |
| :---: | :---: | :---: |
| ![Main Dashboard](screenshots/main.png) | ![Script Editor](screenshots/script.png) | ![Payload Library](screenshots/payload.png) |

<div class="section-header">
  <h2>Documentation</h2>
  <span>Technical architecture &amp; user guides</span>
</div>

<div class="projects-grid">
  <a href="architecture/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-sitemap feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>Architecture &amp; Core Engine</h3>
    <p>High-level system architecture, single-binary Go design, ConfigFS kernel interaction, and package breakdown.</p>
  </a>

  <a href="usb-gadgets/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-usb feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>USB Gadget Functions</h3>
    <p>Technical breakdown of Keyboard, Mouse, Mass Storage, CDC-ECM/RNDIS Network, and ACM Serial interfaces.</p>
  </a>

  <a href="scripting/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-terminal feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>DuckyScript &amp; JS Engines</h3>
    <p>Dual scripting runtime guide, Hak5 Rubber Ducky syntax, Goja JavaScript API reference, and host LED listeners.</p>
  </a>

  <a href="web-dashboard-api/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-globe feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>Web Dashboard &amp; REST API</h3>
    <p>Single Page Application features, REST endpoints reference, and WebSocket real-time event streaming.</p>
  </a>

  <a href="hardware-compatibility/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-microchip feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>Hardware &amp; SBC Matrix</h3>
    <p>USB OTG controller types, DWC2 vs DWC3 IN endpoint hardware limits, supported boards, and binary assets.</p>
  </a>

  <a href="cli-automation/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-gears feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>CLI &amp; Headless Automation</h3>
    <p>Daemon mode flags, headless payload execution, systemd integration, and boot-time crontab automation.</p>
  </a>

  <a href="installation/" class="feature-card">
    <div class="card-top">
      <i class="fa-solid fa-download feature-icon" aria-hidden="true"></i>
      <span class="card-badge">Read →</span>
    </div>
    <h3>Installation &amp; Building</h3>
    <p>One-line automated installation script, cross-compilation instructions, and building from source.</p>
  </a>
</div>
