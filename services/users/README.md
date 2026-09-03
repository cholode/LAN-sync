# Users service migration target

User, friendship, room and membership repositories still live in the root
`repository` package because the compatibility backend accesses them directly.
They will move here after a users gRPC contract is introduced. Keeping this
boundary explicit avoids pretending that an in-process repository is already
a separately deployable service.
