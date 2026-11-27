#include "block/block_cache.h"

#include <iostream>
#include <memory>

int main() {
  for (int reads : {4, 7}) {
    tiny_lsm::BlockCache cache(32, 8);
    auto block = std::make_shared<tiny_lsm::Block>();
    for (int key = 0; key < 8; ++key) {
      cache.put(1, key, block);
      for (int i = 0; i < reads; ++i) {
        cache.get(1, key);
      }
    }
    for (int key = 8; key < 136; ++key) {
      cache.put(1, key, block);
    }
    int hits = 0;
    for (int key = 0; key < 8; ++key) {
      if (cache.get(1, key)) {
        ++hits;
      }
    }
    std::cout << reads << " warm reads: " << hits << "/8\n";
  }
}
