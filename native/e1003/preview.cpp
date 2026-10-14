#include "dashboard.h"
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <iterator>
#include <vector>

int main(int argc,char** argv) {
  if (argc!=3) return 2;
  std::ifstream input(argv[1],std::ios::binary);
  std::vector<uint8_t> font((std::istreambuf_iterator<char>(input)),{});
  if (font.size()!=dashboard::FontSize) return 3;
  std::string payload; char block[4096];
  while (std::cin.good()) {
    std::cin.read(block,sizeof(block)); payload.append(block,size_t(std::cin.gcount()));
    if (payload.size()>dashboard::MaxPayload) return 4;
  }
  JsonDocument doc;
  if (deserializeJson(doc,payload) || !dashboard::valid(doc.as<JsonVariantConst>())) return 5;
  char* end=nullptr; unsigned long page=strtoul(argv[2],&end,10);
  if (!end || *end || page>100000) return 6;
  std::vector<uint8_t> pixels(dashboard::FrameSize);
  dashboard::render(pixels.data(),font.data(),doc.as<JsonVariantConst>(),unsigned(page)%dashboard::pageCount(doc.as<JsonVariantConst>()));
  std::cout.write(reinterpret_cast<const char*>(pixels.data()),pixels.size());
  return std::cout.good()?0:7;
}
