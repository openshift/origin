function $each(map, func) {
  for (var key in map) {
    func(key, map[key]);
  }
};
