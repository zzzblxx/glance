#include <Arduino.h>
#include <Seeed_GFX.h>
#include <panel/Panel_EPaper.h>
#include <mbedtls/sha256.h>
#include "dashboard.h"

// External USB-C is CH340K / UART (Seeed E10xx Arduino peripherals guide).
static constexpr uint32_t Baud = 115200, ReceiveTimeout = 5000;
static constexpr size_t Chunk = 1024;
static constexpr int PreviousButton = 5, NextButton = 3;
extern const uint8_t fontStart[] asm("_binary_generated_unifont16_bin_start");
extern const uint8_t fontEnd[] asm("_binary_generated_unifont16_bin_end");

struct PSRAMAllocator : ArduinoJson::Allocator {
  void *allocate(size_t n) override { return heap_caps_malloc(n, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT); }
  void deallocate(void *p) override { free(p); }
  void *reallocate(void *p, size_t n) override { return heap_caps_realloc(p, n, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT); }
} jsonAllocator;
JsonDocument active(&jsonAllocator);
Seeed_GFX display(Seeed_Product::reTerminal_E1003);
bool displayReady = false, hasSnapshot = false;
String deviceID, snapshotSHA, frameSHA, expectedSHA, line;
uint8_t *receiving = nullptr;
size_t expected = 0, received = 0;
uint32_t lastByte = 0;
unsigned page = 0;
bool lineOverflow = false;

static void reply(JsonDocument &doc) { serializeJson(doc, Serial1); Serial1.write('\n'); }
static void reject(const char *reason) {
  JsonDocument doc; doc["ok"] = false; doc["error"] = reason; reply(doc);
}
static bool validSHA(const String &s) {
  if (s.length() != 64) return false;
  for (char c : s) if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))) return false;
  return true;
}
static String digest(const uint8_t *data, size_t size) {
  uint8_t bytes[32];
  if (mbedtls_sha256(data, size, bytes, 0) != 0) return "";
  char hex[65];
  for (size_t i = 0; i < 32; ++i) snprintf(hex + i * 2, 3, "%02x", bytes[i]);
  return String(hex);
}
static bool initDisplay() {
  if (displayReady) return true;
  if (!display.begin() || !display.panel().configure(PanelMode::Gray16)) return false;
  displayReady = display.width() == dashboard::NativeWidth && display.height() == dashboard::NativeHeight;
  return displayReady;
}
static bool show(JsonDocument &doc, unsigned nextPage) {
  if (static_cast<size_t>(fontEnd - fontStart) != dashboard::FontSize) return false;
  auto *pixels = static_cast<uint8_t *>(heap_caps_malloc(dashboard::FrameSize, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT));
  if (!pixels) return false;
  dashboard::render(pixels, fontStart, doc.as<JsonVariantConst>(), nextPage);
  String hash = digest(pixels, dashboard::FrameSize);
  bool ok = !hash.isEmpty() && hash == frameSHA;
  if (!ok && !hash.isEmpty() && initDisplay()) {
    auto &panel = static_cast<Panel_EPaper &>(display.panel());
    if (panel.frameBuffer() && panel.frameBufferSize() == dashboard::FrameSize) {
      memcpy(panel.frameBuffer(), pixels, dashboard::FrameSize);
      ok = static_cast<bool>(display.refresh());
    }
  }
  free(pixels);
  if (ok) { frameSHA = hash; page = nextPage; }
  return ok;
}
static void discardReceive() {
  free(receiving); receiving = nullptr; expected = received = 0; expectedSHA = "";
}
static void applied() {
  JsonDocument doc;
  doc["ok"] = true; doc["op"] = "applied"; doc["snapshot"] = snapshotSHA;
  doc["frame"] = frameSHA; doc["page"] = page; reply(doc);
}
static void finishReceive() {
  if (digest(receiving, expected) != expectedSHA) { discardReceive(); reject("checksum"); return; }
  JsonDocument candidate(&jsonAllocator);
  // Read-only input: strings are owned before the receive buffer is freed.
  auto err = deserializeJson(candidate, static_cast<const uint8_t *>(receiving), expected);
  if (err || !dashboard::valid(candidate.as<JsonVariantConst>())) { discardReceive(); reject("snapshot"); return; }
  unsigned nextPage = page % dashboard::pageCount(candidate.as<JsonVariantConst>());
  if (!show(candidate, nextPage)) { discardReceive(); reject("display"); return; }
  active = std::move(candidate); hasSnapshot = true; snapshotSHA = expectedSHA;
  discardReceive(); applied();
}
static void command() {
  JsonDocument doc;
  if (deserializeJson(doc, line) || doc["protocol"] != 2) { reject("protocol"); return; }
  if (doc["op"] == "hello") {
    JsonDocument out;
    out["ok"] = true; out["op"] = "hello"; out["protocol"] = 2;
    out["device"] = "e1003"; out["id"] = deviceID; out["renderer"] = dashboard::RendererVersion;
    out["snapshot"] = snapshotSHA; out["frame"] = frameSHA; out["page"] = page;
    reply(out); return;
  }
  if (doc["op"] != "begin" || !doc["bytes"].is<unsigned>() ||
      doc["bytes"].as<unsigned>() == 0 || doc["bytes"].as<unsigned>() > dashboard::MaxPayload ||
      !doc["sha256"].is<const char *>() || !validSHA(doc["sha256"].as<String>())) { reject("begin"); return; }
  expected = doc["bytes"].as<size_t>(); expectedSHA = doc["sha256"].as<String>();
  receiving = static_cast<uint8_t *>(heap_caps_malloc(expected, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT));
  if (!receiving) { discardReceive(); reject("memory"); return; }
  received = 0; lastByte = millis();
  JsonDocument out; out["ok"] = true; out["op"] = "ready"; reply(out);
}
static void serviceSerial() {
  if (receiving && millis() - lastByte > ReceiveTimeout) { discardReceive(); reject("timeout"); }
  while (Serial1.available()) {
    if (receiving) {
      receiving[received++] = static_cast<uint8_t>(Serial1.read()); lastByte = millis();
      if (received == expected) finishReceive();
      else if (received % Chunk == 0) {
        JsonDocument out; out["ok"] = true; out["op"] = "chunk"; out["offset"] = received; reply(out);
      }
    } else {
      char c = Serial1.read();
      if (c == '\n') {
        if (lineOverflow) reject("line_length"); else if (!line.isEmpty()) command();
        line = ""; lineOverflow = false;
      } else if (c != '\r') {
        if (line.length() < 512 && !lineOverflow) line += c;
        else { line = ""; lineOverflow = true; }
      }
    }
  }
}
static void buttons() {
  static int previous = HIGH, next = HIGH, stablePrevious = HIGH, stableNext = HIGH;
  static uint32_t changed = 0;
  int p = digitalRead(PreviousButton), n = digitalRead(NextButton);
  if (p != previous || n != next) { previous = p; next = n; changed = millis(); }
  if (millis() - changed < 60) return;
  bool back = stablePrevious == HIGH && p == LOW, forward = stableNext == HIGH && n == LOW;
  stablePrevious = p; stableNext = n;
  if (!hasSnapshot || receiving || (!back && !forward)) return;
  unsigned pages = dashboard::pageCount(active.as<JsonVariantConst>());
  unsigned target = back ? (page + pages - 1) % pages : (page + 1) % pages;
  if (target != page) show(active, target);
}
void setup() {
  // Neither enable Wi-Fi nor erase the retained panel on boot.
  Serial1.setRxBufferSize(2048);
  Serial1.begin(Baud, SERIAL_8N1, 44, 43);
  line.reserve(512);
  char id[13]; snprintf(id, sizeof(id), "%012llx", ESP.getEfuseMac() & 0xffffffffffffULL); deviceID = id;
  pinMode(PreviousButton, INPUT_PULLUP); pinMode(NextButton, INPUT_PULLUP);
}
void loop() { serviceSerial(); buttons(); delay(1); }
