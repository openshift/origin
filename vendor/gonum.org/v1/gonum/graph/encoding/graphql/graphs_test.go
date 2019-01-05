// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graphql

const (
	starwars = `{
  "movie": [
    {
      "_uid_": "0xa3cff1a4c3ef3bb6",
      "director": [
        {
          "_uid_": "0x4a7d0b5fe91e78a4",
          "name": "Irvin Kernshner"
        }
      ],
      "name": "Star Wars: Episode V - The Empire Strikes Back",
      "release_date": "1980-05-21T00:00:00Z",
      "revenue": 534000000,
      "running_time": 124,
      "starring": [
        {
          "_uid_": "0x312de17a7ee89f9",
          "name": "Luke Skywalker"
        },
        {
          "_uid_": "0x3da8d1dcab1bb381",
          "name": "Han Solo"
        },
        {
          "_uid_": "0x718337b9dcbaa7d9",
          "name": "Princess Leia"
        }
      ]
    },
    {
      "_uid_": "0xb39aa14d66aedad5",
      "director": [
        {
          "_uid_": "0x8a10d5a2611fd03f",
          "name": "Richard Marquand"
        }
      ],
      "name": "Star Wars: Episode VI - Return of the Jedi",
      "release_date": "1983-05-25T00:00:00Z",
      "revenue": 572000000,
      "running_time": 131,
      "starring": [
        {
          "_uid_": "0x312de17a7ee89f9",
          "name": "Luke Skywalker"
        },
        {
          "_uid_": "0x3da8d1dcab1bb381",
          "name": "Han Solo"
        },
        {
          "_uid_": "0x718337b9dcbaa7d9",
          "name": "Princess Leia"
        }
      ]
    }
  ]
}
`

	dgraphTutorialMissingIDs = `{
    "everyone": [
        {
            "_uid_": "0xfd90205a458151f",
            "friend": [
                {
                    "_uid_": "0x52a80955d40ec819",
                    "friend": [
                        {
                            "_uid_": "0xfd90205a458151f",
                            "age": 39,
                            "friend": [
                                {
                                    "age": 35,
                                    "name": "Amit"
                                },
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                },
                                {
                                    "age": 55,
                                    "name": "Sarah"
                                },
                                {
                                    "age": 35,
                                    "name": "Artyom"
                                },
                                {
                                    "age": 19,
                                    "name": "Catalina"
                                }
                            ],
                            "name": "Michael",
                            "owns_pet": [
                                {
                                    "name": "Rammy the sheep"
                                }
                            ]
                        },
                        {
                            "_uid_": "0x5e9ad1cd9466228c",
                            "age": 24,
                            "friend": [
                                {
                                    "age": 35,
                                    "name": "Amit"
                                },
                                {
                                    "age": 19,
                                    "name": "Catalina"
                                },
                                {
                                    "name": "Hyung Sin"
                                }
                            ],
                            "name": "Sang Hyun",
                            "owns_pet": [
                                {
                                    "name": "Goldie"
                                }
                            ]
                        },
                        {
                            "_uid_": "0x99b74c1b5ab100ec",
                            "age": 35,
                            "name": "Artyom"
                        }
                    ],
                    "name": "Amit"
                },
                {
                    "_uid_": "0x5e9ad1cd9466228c",
                    "friend": [
                        {
                            "_uid_": "0x52a80955d40ec819",
                            "age": 35,
                            "friend": [
                                {
                                    "age": 39,
                                    "name": "Michael"
                                },
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                },
                                {
                                    "age": 35,
                                    "name": "Artyom"
                                }
                            ],
                            "name": "Amit"
                        },
                        {
                            "_uid_": "0xb9e12a67e34d6acc",
                            "age": 19,
                            "friend": [
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                }
                            ],
                            "name": "Catalina",
                            "owns_pet": [
                                {
                                    "name": "Perro"
                                }
                            ]
                        },
                        {
                            "_uid_": "0xf92d7dbe272d680b",
                            "friend": [
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                }
                            ],
                            "name": "Hyung Sin"
                        }
                    ],
                    "name": "Sang Hyun"
                },
                {
                    "_uid_": "0x892a6da7ee1fbdec",
                    "name": "Sarah"
                },
                {
                    "_uid_": "0x99b74c1b5ab100ec",
                    "name": "Artyom"
                },
                {
                    "_uid_": "0xb9e12a67e34d6acc",
                    "friend": [
                        {
                            "_uid_": "0x5e9ad1cd9466228c",
                            "age": 24,
                            "friend": [
                                {
                                    "age": 35,
                                    "name": "Amit"
                                },
                                {
                                    "age": 19,
                                    "name": "Catalina"
                                },
                                {
                                    "name": "Hyung Sin"
                                }
                            ],
                            "name": "Sang Hyun",
                            "owns_pet": [
                                {
                                    "name": "Goldie"
                                }
                            ]
                        }
                    ],
                    "name": "Catalina"
                }
            ],
            "name": "Michael"
        },
        {
            "_uid_": "0x52a80955d40ec819",
            "friend": [
                {
                    "_uid_": "0xfd90205a458151f",
                    "friend": [
                        {
                            "_uid_": "0x52a80955d40ec819",
                            "age": 35,
                            "friend": [
                                {
                                    "age": 39,
                                    "name": "Michael"
                                },
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                },
                                {
                                    "age": 35,
                                    "name": "Artyom"
                                }
                            ],
                            "name": "Amit"
                        },
                        {
                            "_uid_": "0x5e9ad1cd9466228c",
                            "age": 24,
                            "friend": [
                                {
                                    "age": 35,
                                    "name": "Amit"
                                },
                                {
                                    "age": 19,
                                    "name": "Catalina"
                                },
                                {
                                    "name": "Hyung Sin"
                                }
                            ],
                            "name": "Sang Hyun",
                            "owns_pet": [
                                {
                                    "name": "Goldie"
                                }
                            ]
                        },
                        {
                            "_uid_": "0x892a6da7ee1fbdec",
                            "age": 55,
                            "name": "Sarah"
                        },
                        {
                            "_uid_": "0x99b74c1b5ab100ec",
                            "age": 35,
                            "name": "Artyom"
                        },
                        {
                            "_uid_": "0xb9e12a67e34d6acc",
                            "age": 19,
                            "friend": [
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                }
                            ],
                            "name": "Catalina",
                            "owns_pet": [
                                {
                                    "name": "Perro"
                                }
                            ]
                        }
                    ],
                    "name": "Michael"
                },
                {
                    "_uid_": "0x5e9ad1cd9466228c",
                    "friend": [
                        {
                            "_uid_": "0x52a80955d40ec819",
                            "age": 35,
                            "friend": [
                                {
                                    "age": 39,
                                    "name": "Michael"
                                },
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                },
                                {
                                    "age": 35,
                                    "name": "Artyom"
                                }
                            ],
                            "name": "Amit"
                        },
                        {
                            "_uid_": "0xb9e12a67e34d6acc",
                            "age": 19,
                            "friend": [
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                }
                            ],
                            "name": "Catalina",
                            "owns_pet": [
                                {
                                    "name": "Perro"
                                }
                            ]
                        },
                        {
                            "_uid_": "0xf92d7dbe272d680b",
                            "friend": [
                                {
                                    "age": 24,
                                    "name": "Sang Hyun"
                                }
                            ],
                            "name": "Hyung Sin"
                        }
                    ],
                    "name": "Sang Hyun"
                },
                {
                    "_uid_": "0x99b74c1b5ab100ec",
                    "name": "Artyom"
                }
            ],
            "name": "Amit"
        }
    ]
}
`

	dgraphTutorial = `{
  "everyone": [
    {
      "_uid_": "0xfd90205a458151f",
      "friend": [
        {
          "_uid_": "0x52a80955d40ec819",
          "friend": [
            {
              "_uid_": "0xfd90205a458151f",
              "age": 39,
              "friend": [
                {
                  "_uid_": "0x52a80955d40ec819",
                  "age": 35,
                  "friend": [
                    {
                      "_uid_": "0xfd90205a458151f"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    }
                  ],
                  "name": "Amit"
                },
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                },
                {
                  "_uid_": "0x892a6da7ee1fbdec",
                  "age": 55,
                  "name": "Sarah"
                },
                {
                  "_uid_": "0x99b74c1b5ab100ec",
                  "age": 35,
                  "name": "Artyom"
                },
                {
                  "_uid_": "0xb9e12a67e34d6acc",
                  "age": 19,
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Catalina",
                  "owns_pet": [
                    {
                      "_uid_": "0xbf104824c777525d"
                    }
                  ]
                }
              ],
              "name": "Michael",
              "owns_pet": [
                {
                  "_uid_": "0x37734fcf0a6fcc69",
                  "name": "Rammy the sheep"
                }
              ]
            },
            {
              "_uid_": "0x5e9ad1cd9466228c",
              "age": 24,
              "friend": [
                {
                  "_uid_": "0x52a80955d40ec819",
                  "age": 35,
                  "friend": [
                    {
                      "_uid_": "0xfd90205a458151f"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    }
                  ],
                  "name": "Amit"
                },
                {
                  "_uid_": "0xb9e12a67e34d6acc",
                  "age": 19,
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Catalina",
                  "owns_pet": [
                    {
                      "_uid_": "0xbf104824c777525d"
                    }
                  ]
                },
                {
                  "_uid_": "0xf92d7dbe272d680b",
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Hyung Sin"
                }
              ],
              "name": "Sang Hyun",
              "owns_pet": [
                {
                  "_uid_": "0xf590a923ea1fccaa",
                  "name": "Goldie"
                }
              ]
            },
            {
              "_uid_": "0x99b74c1b5ab100ec",
              "age": 35,
              "name": "Artyom"
            }
          ],
          "name": "Amit"
        },
        {
          "_uid_": "0x5e9ad1cd9466228c",
          "friend": [
            {
              "_uid_": "0x52a80955d40ec819",
              "age": 35,
              "friend": [
                {
                  "_uid_": "0xfd90205a458151f",
                  "age": 39,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x892a6da7ee1fbdec"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    }
                  ],
                  "name": "Michael",
                  "owns_pet": [
                    {
                      "_uid_": "0x37734fcf0a6fcc69"
                    }
                  ]
                },
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                },
                {
                  "_uid_": "0x99b74c1b5ab100ec",
                  "age": 35,
                  "name": "Artyom"
                }
              ],
              "name": "Amit"
            },
            {
              "_uid_": "0xb9e12a67e34d6acc",
              "age": 19,
              "friend": [
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                }
              ],
              "name": "Catalina",
              "owns_pet": [
                {
                  "_uid_": "0xbf104824c777525d",
                  "name": "Perro"
                }
              ]
            },
            {
              "_uid_": "0xf92d7dbe272d680b",
              "friend": [
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                }
              ],
              "name": "Hyung Sin"
            }
          ],
          "name": "Sang Hyun"
        },
        {
          "_uid_": "0x892a6da7ee1fbdec",
          "name": "Sarah"
        },
        {
          "_uid_": "0x99b74c1b5ab100ec",
          "name": "Artyom"
        },
        {
          "_uid_": "0xb9e12a67e34d6acc",
          "friend": [
            {
              "_uid_": "0x5e9ad1cd9466228c",
              "age": 24,
              "friend": [
                {
                  "_uid_": "0x52a80955d40ec819",
                  "age": 35,
                  "friend": [
                    {
                      "_uid_": "0xfd90205a458151f"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    }
                  ],
                  "name": "Amit"
                },
                {
                  "_uid_": "0xb9e12a67e34d6acc",
                  "age": 19,
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Catalina",
                  "owns_pet": [
                    {
                      "_uid_": "0xbf104824c777525d"
                    }
                  ]
                },
                {
                  "_uid_": "0xf92d7dbe272d680b",
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Hyung Sin"
                }
              ],
              "name": "Sang Hyun",
              "owns_pet": [
                {
                  "_uid_": "0xf590a923ea1fccaa",
                  "name": "Goldie"
                }
              ]
            }
          ],
          "name": "Catalina"
        }
      ],
      "name": "Michael"
    },
    {
      "_uid_": "0x52a80955d40ec819",
      "friend": [
        {
          "_uid_": "0xfd90205a458151f",
          "friend": [
            {
              "_uid_": "0x52a80955d40ec819",
              "age": 35,
              "friend": [
                {
                  "_uid_": "0xfd90205a458151f",
                  "age": 39,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x892a6da7ee1fbdec"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    }
                  ],
                  "name": "Michael",
                  "owns_pet": [
                    {
                      "_uid_": "0x37734fcf0a6fcc69"
                    }
                  ]
                },
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                },
                {
                  "_uid_": "0x99b74c1b5ab100ec",
                  "age": 35,
                  "name": "Artyom"
                }
              ],
              "name": "Amit"
            },
            {
              "_uid_": "0x5e9ad1cd9466228c",
              "age": 24,
              "friend": [
                {
                  "_uid_": "0x52a80955d40ec819",
                  "age": 35,
                  "friend": [
                    {
                      "_uid_": "0xfd90205a458151f"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    }
                  ],
                  "name": "Amit"
                },
                {
                  "_uid_": "0xb9e12a67e34d6acc",
                  "age": 19,
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Catalina",
                  "owns_pet": [
                    {
                      "_uid_": "0xbf104824c777525d"
                    }
                  ]
                },
                {
                  "_uid_": "0xf92d7dbe272d680b",
                  "friend": [
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    }
                  ],
                  "name": "Hyung Sin"
                }
              ],
              "name": "Sang Hyun",
              "owns_pet": [
                {
                  "_uid_": "0xf590a923ea1fccaa",
                  "name": "Goldie"
                }
              ]
            },
            {
              "_uid_": "0x892a6da7ee1fbdec",
              "age": 55,
              "name": "Sarah"
            },
            {
              "_uid_": "0x99b74c1b5ab100ec",
              "age": 35,
              "name": "Artyom"
            },
            {
              "_uid_": "0xb9e12a67e34d6acc",
              "age": 19,
              "friend": [
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                }
              ],
              "name": "Catalina",
              "owns_pet": [
                {
                  "_uid_": "0xbf104824c777525d",
                  "name": "Perro"
                }
              ]
            }
          ],
          "name": "Michael"
        },
        {
          "_uid_": "0x5e9ad1cd9466228c",
          "friend": [
            {
              "_uid_": "0x52a80955d40ec819",
              "age": 35,
              "friend": [
                {
                  "_uid_": "0xfd90205a458151f",
                  "age": 39,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0x5e9ad1cd9466228c"
                    },
                    {
                      "_uid_": "0x892a6da7ee1fbdec"
                    },
                    {
                      "_uid_": "0x99b74c1b5ab100ec"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    }
                  ],
                  "name": "Michael",
                  "owns_pet": [
                    {
                      "_uid_": "0x37734fcf0a6fcc69"
                    }
                  ]
                },
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                },
                {
                  "_uid_": "0x99b74c1b5ab100ec",
                  "age": 35,
                  "name": "Artyom"
                }
              ],
              "name": "Amit"
            },
            {
              "_uid_": "0xb9e12a67e34d6acc",
              "age": 19,
              "friend": [
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                }
              ],
              "name": "Catalina",
              "owns_pet": [
                {
                  "_uid_": "0xbf104824c777525d",
                  "name": "Perro"
                }
              ]
            },
            {
              "_uid_": "0xf92d7dbe272d680b",
              "friend": [
                {
                  "_uid_": "0x5e9ad1cd9466228c",
                  "age": 24,
                  "friend": [
                    {
                      "_uid_": "0x52a80955d40ec819"
                    },
                    {
                      "_uid_": "0xb9e12a67e34d6acc"
                    },
                    {
                      "_uid_": "0xf92d7dbe272d680b"
                    }
                  ],
                  "name": "Sang Hyun",
                  "owns_pet": [
                    {
                      "_uid_": "0xf590a923ea1fccaa"
                    }
                  ]
                }
              ],
              "name": "Hyung Sin"
            }
          ],
          "name": "Sang Hyun"
        },
        {
          "_uid_": "0x99b74c1b5ab100ec",
          "name": "Artyom"
        }
      ],
      "name": "Amit"
    }
  ]
}
`
)
