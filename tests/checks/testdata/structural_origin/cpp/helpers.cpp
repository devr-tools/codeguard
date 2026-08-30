struct Response {
  int code;
};

struct ResponseBuilder {
  int code;
  void set_code(int value) { code = value; }
  Response build() { return Response{code}; }
};

Response BuildLocalResponse() {
  ResponseBuilder builder{};
  builder.set_code(200);
  return builder.build();
}

struct DbRow {
  int integer(int column) const;
};

int DbRow::integer(int column) const {
  int value = 0;
  value = column;
  return value;
}

struct Token {
  int value;
};

struct Counter {
  Token state;
  int CurrentReceiver() {
    state.value++;
    return state.value;
  }
};

int CurrentReference(Token& input) {
  input.value++;
  return input.value;
}

Token sharedToken;

int CurrentGlobal() {
  sharedToken.value++;
  return sharedToken.value;
}

Token* escapedToken;

int CurrentEscaped() {
  Token item{};
  if (true) escapedToken = &item;
  item.value++;
  return item.value;
}

int CurrentUnresolvedCpp() {
  mystery.update();
  return 1;
}
